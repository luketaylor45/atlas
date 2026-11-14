package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"sort"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/luketaylor45/atlas/core/internal/database"
	"github.com/luketaylor45/atlas/core/internal/models"
	"github.com/luketaylor45/atlas/core/internal/utils"
	"gorm.io/gorm"
)

type ImportEggRequest struct {
	NestID  uint   `json:"nest_id"`
	Content string `json:"content"` // JSON string of the egg (Atlas or Pterodactyl format)
}

// importedVariable accepts both formats. The visibility flags are pointers so
// a file that leaves them out means "yes" rather than "no".
type importedVariable struct {
	Name         string `json:"name"`
	Description  string `json:"description"`
	EnvVariable  string `json:"env_variable"`
	DefaultValue string `json:"default_value"`
	UserViewable *bool  `json:"user_viewable"`
	UserEditable *bool  `json:"user_editable"`
	Rules        string `json:"rules"`
	InputType    string `json:"input_type"`
}

func flagOr(value *bool, fallback bool) bool {
	if value == nil {
		return fallback
	}
	return *value
}

// parseEggImport turns an Atlas or Pterodactyl egg file into an egg and its
// variables, without touching the database.
func parseEggImport(content string) (*models.Egg, []models.EggVariable, error) {
	// The two formats share field names with different shapes (docker_images
	// is a list in one and a map in the other), so the file is inspected
	// before it is decoded into either.
	var probe map[string]json.RawMessage
	if err := json.Unmarshal([]byte(content), &probe); err != nil {
		return nil, nil, errors.New("the file is not valid JSON")
	}

	var egg models.Egg
	var variables []importedVariable

	if _, isAtlas := probe["startup_command"]; isAtlas {
		var file struct {
			Name            string             `json:"name"`
			Author          string             `json:"author"`
			Description     string             `json:"description"`
			DockerImages    []string           `json:"docker_images"`
			StartupCommand  string             `json:"startup_command"`
			StopCommand     string             `json:"stop_command"`
			ScriptInstall   string             `json:"script_install"`
			ScriptContainer string             `json:"script_container"`
			ScriptEntry     string             `json:"script_entry"`
			Ports           []models.EggPort   `json:"ports"`
			Variables       []importedVariable `json:"variables"`
		}
		if err := json.Unmarshal([]byte(content), &file); err != nil {
			return nil, nil, errors.New("the file is not a valid Atlas egg: " + err.Error())
		}
		images, _ := json.Marshal(file.DockerImages)
		egg = models.Egg{
			Name:            file.Name,
			Author:          file.Author,
			Description:     file.Description,
			DockerImages:    string(images),
			StartupCommand:  file.StartupCommand,
			StopCommand:     file.StopCommand,
			ScriptInstall:   file.ScriptInstall,
			ScriptContainer: file.ScriptContainer,
			ScriptEntry:     file.ScriptEntry,
		}
		if len(file.Ports) > 0 {
			ports, _ := json.Marshal(file.Ports)
			egg.Ports = string(ports)
		}
		variables = file.Variables
	} else {
		var file struct {
			Name         string            `json:"name"`
			Author       string            `json:"author"`
			Description  string            `json:"description"`
			DockerImages map[string]string `json:"docker_images"`
			Startup      string            `json:"startup"`
			Config       struct {
				Stop string `json:"stop"`
			} `json:"config"`
			Scripts struct {
				Installation struct {
					Script     string `json:"script"`
					Container  string `json:"container"`
					Entrypoint string `json:"entrypoint"`
				} `json:"installation"`
			} `json:"scripts"`
			Variables []importedVariable `json:"variables"`
		}
		if err := json.Unmarshal([]byte(content), &file); err != nil {
			return nil, nil, errors.New("the file is not a valid Pterodactyl egg: " + err.Error())
		}

		// A JSON object has no order, and the first image becomes the
		// default. Sorting by label, newest first, makes that choice stable
		// and matches how these eggs list their images ("Java 21", "Java 17").
		labels := make([]string, 0, len(file.DockerImages))
		for label := range file.DockerImages {
			labels = append(labels, label)
		}
		sort.Sort(sort.Reverse(naturalOrder(labels)))
		ordered := make([]string, 0, len(labels))
		for _, label := range labels {
			ordered = append(ordered, file.DockerImages[label])
		}
		images, _ := json.Marshal(ordered)

		egg = models.Egg{
			Name:            file.Name,
			Author:          file.Author,
			Description:     file.Description,
			DockerImages:    string(images),
			StartupCommand:  file.Startup,
			StopCommand:     file.Config.Stop,
			ScriptInstall:   file.Scripts.Installation.Script,
			ScriptContainer: file.Scripts.Installation.Container,
			ScriptEntry:     file.Scripts.Installation.Entrypoint,
		}
		variables = file.Variables
	}

	egg.Name = strings.TrimSpace(egg.Name)
	if egg.Name == "" {
		return nil, nil, errors.New("the egg has no name")
	}
	if strings.TrimSpace(egg.StartupCommand) == "" {
		return nil, nil, errors.New("the egg has no startup command")
	}
	if len(eggImages(&egg)) == 0 {
		return nil, nil, errors.New("the egg lists no Docker images")
	}
	if egg.Author == "" {
		egg.Author = "Imported"
	}

	seen := map[string]bool{}
	result := make([]models.EggVariable, 0, len(variables))
	for _, variable := range variables {
		env := strings.TrimSpace(variable.EnvVariable)
		if env == "" || seen[env] {
			continue
		}
		seen[env] = true

		inputType := variable.InputType
		if inputType == "" {
			inputType = "text"
		}
		result = append(result, models.EggVariable{
			Name:                variable.Name,
			Description:         variable.Description,
			EnvironmentVariable: env,
			DefaultValue:        variable.DefaultValue,
			UserViewable:        flagOr(variable.UserViewable, true),
			UserEditable:        flagOr(variable.UserEditable, true),
			Rules:               variable.Rules,
			InputType:           inputType,
		})
	}
	return &egg, result, nil
}

// naturalOrder sorts labels so that numbers inside them compare by value
// ("Java 8" before "Java 17").
type naturalOrder []string

func (n naturalOrder) Len() int      { return len(n) }
func (n naturalOrder) Swap(i, j int) { n[i], n[j] = n[j], n[i] }
func (n naturalOrder) Less(i, j int) bool {
	a, b := n[i], n[j]
	for a != "" && b != "" {
		aDigits, bDigits := leadingDigits(a), leadingDigits(b)
		if aDigits > 0 && bDigits > 0 {
			aNumber, bNumber := strings.TrimLeft(a[:aDigits], "0"), strings.TrimLeft(b[:bDigits], "0")
			if len(aNumber) != len(bNumber) {
				return len(aNumber) < len(bNumber)
			}
			if aNumber != bNumber {
				return aNumber < bNumber
			}
			a, b = a[aDigits:], b[bDigits:]
			continue
		}
		if a[0] != b[0] {
			return a[0] < b[0]
		}
		a, b = a[1:], b[1:]
	}
	return len(a) < len(b)
}

func leadingDigits(value string) int {
	count := 0
	for count < len(value) && value[count] >= '0' && value[count] <= '9' {
		count++
	}
	return count
}

// ImportEgg adds an egg from an Atlas or Pterodactyl egg file.
func ImportEgg(c *gin.Context) {
	var req ImportEggRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	var nest models.Nest
	if err := database.DB.First(&nest, req.NestID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Choose a nest to import the egg into"})
		return
	}

	egg, variables, err := parseEggImport(req.Content)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	egg.UUID = uuid.New().String()
	egg.NestID = nest.ID
	egg.Available = true
	egg.Source = "import"

	// The egg and its variables are created together or not at all.
	err = database.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(egg).Error; err != nil {
			return err
		}
		for i := range variables {
			variables[i].EggID = egg.ID
			viewable, editable := variables[i].UserViewable, variables[i].UserEditable
			if err := tx.Create(&variables[i]).Error; err != nil {
				return err
			}
			// Written separately: a false value would otherwise be replaced
			// by the column default on insert.
			if err := tx.Model(&models.EggVariable{}).Where("id = ?", variables[i].ID).Updates(map[string]interface{}{
				"user_viewable": viewable,
				"user_editable": editable,
			}).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to import the egg"})
		return
	}

	utils.LogActivity(c, 0, "create", "egg", "Imported egg: "+egg.Name, nil)

	database.DB.Preload("Variables").First(egg, egg.ID)
	c.JSON(http.StatusCreated, egg)
}
