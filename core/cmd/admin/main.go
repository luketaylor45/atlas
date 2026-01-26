package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/luketaylor45/atlas/core/internal/config"
	"github.com/luketaylor45/atlas/core/internal/database"
	"github.com/luketaylor45/atlas/core/internal/models"
	"golang.org/x/crypto/bcrypt"
)

func main() {
	cmd := flag.String("action", "create", "Actions: create, reset-password")
	flag.Parse()

	config.LoadConfig()
	database.Connect()

	reader := bufio.NewReader(os.Stdin)

	switch *cmd {
	case "create":
		fmt.Print("Enter Admin Username: ")
		username, _ := reader.ReadString('\n')
		username = strings.TrimSpace(username)

		fmt.Print("Enter Admin Password: ")
		password, _ := reader.ReadString('\n')
		password = strings.TrimSpace(password)

		hashedPassword, _ := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)

		user := models.User{
			Username: username,
			Password: string(hashedPassword),
			IsAdmin:  true,
		}

		if err := database.DB.Create(&user).Error; err != nil {
			log.Fatalf("Failed to create admin: %v", err)
		}
		fmt.Printf("Successfully created admin: %s\n", username)

	case "reset-password":
		fmt.Print("Enter Username to Reset: ")
		username, _ := reader.ReadString('\n')
		username = strings.TrimSpace(username)

		var user models.User
		if err := database.DB.Where("username = ?", username).First(&user).Error; err != nil {
			log.Fatalf("User not found: %s", username)
		}

		fmt.Print("Enter New Password: ")
		password, _ := reader.ReadString('\n')
		password = strings.TrimSpace(password)

		hashedPassword, _ := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
		user.Password = string(hashedPassword)

		database.DB.Save(&user)
		fmt.Printf("Successfully reset password for %s\n", username)

	default:
		fmt.Println("Invalid action. Use -action=create or -action=reset-password")
	}
}
