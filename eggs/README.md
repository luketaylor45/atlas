# Atlas egg catalog

Every `.json` file under this directory is an egg: the recipe for installing
and running one kind of server. Core loads the catalog on start. An egg that
an admin edited in the panel is left alone; an egg whose file is removed is
hidden, not deleted.

## Layout

```
eggs/
  _scripts/            shared install scripts (directories starting with "_" are not nests)
  games/               top-level nest (nest.json gives it a name and description)
    minecraft/         sub-nest
      nest.json
      paper.json       an egg
```

Nests go two levels deep at most.

## Egg file

```jsonc
{
  "uuid": "atlas-mc-paper",            // stable identifier, must start with "atlas-"
  "name": "Paper",
  "description": "High performance Spigot fork.",
  "docker_images": ["ghcr.io/ptero-eggs/yolks:java_25"],   // first is the default
  "startup_command": "java -Xmx{{SERVER_MEMORY}}M -jar {{SERVER_JARFILE}}",
  "stop_command": "stop",              // typed into the console; "^C" sends an interrupt
  "startup_done": ["Done ("],          // console text that means the server is ready
  "config_files": { },                 // files rewritten before every start, see below
  "script_install": "#!/bin/bash\n…",  // install script, inline …
  "script_install_files": ["_scripts/steamcmd.sh", "_scripts/palworld.sh"],  // … or files, joined in order
  "script_container": "ghcr.io/pterodactyl/installers:debian",
  "script_entry": "bash",
  "ports": [                           // ports besides the primary one
    { "name": "Query", "env": "QUERY_PORT", "default": 27016, "protocol": "udp" },
    { "name": "Steam", "env": "STEAM_PORT", "default": 26901, "protocol": "udp", "offset": 1 }
  ],
  "variables": [
    {
      "name": "Minecraft Version",
      "env_variable": "MINECRAFT_VERSION",
      "default_value": "latest",
      "description": "Version to install, or 'latest'.",
      "user_viewable": true,           // both default to true
      "user_editable": true,
      "rules": "required|string|max:20"
    }
  ]
}
```

`script_install_files` paths are relative to this directory. When both it and
`script_install` are present, the inline script is appended last.

### Startup command

`{{NAME}}` is replaced with the value of the variable `NAME`. Every variable is
also an environment variable in the container. Atlas always provides:

| Variable | Value |
| --- | --- |
| `SERVER_PORT` | the primary port |
| `SERVER_MEMORY` | memory limit in MB |
| `SERVER_IP` | `0.0.0.0` |
| `SERVER_UUID` | the service identifier |
| one per entry in `ports` | the port allocated for it |

A port with an `offset` is one the game works out for itself from its primary
port (Valheim's query port is always its game port plus one). Atlas then
allocates exactly the primary port plus the offset, moves it when the primary
port changes, and refuses a primary port whose partner is already in use.

The primary port is published on TCP and UDP. The game runs in
`/home/container` as an unprivileged user; the installer runs as root with the
same directory at `/mnt/server`.

### `startup_done`

A list of strings. The server shows as *Starting* until a console line
contains one of them, then *Running*. Prefix a string with `regex:` to match a
regular expression. Leave it out and the server shows as running as soon as
its process starts.

### `config_files`

Same shape as Pterodactyl's `config.files`:

```json
{
  "server.properties": {
    "parser": "properties",
    "find": {
      "server-port": "{{server.build.default.port}}",
      "motd": "{{server.build.env.MOTD}}"
    }
  }
}
```

Parsers: `properties`, `ini`, `json`, `yaml`, `xml` and `file` (replaces whole
lines that start with the key). A file that does not exist yet is created for
`properties`, `ini`, `json` and `yaml`. For `json` and `yaml` the keys are
dotted paths (`network.port`); for `ini`, `section.key`.

Placeholders: `{{server.build.default.port}}`, `{{server.build.default.ip}}`,
`{{server.build.memory}}`, `{{server.build.env.NAME}}`, `{{env.NAME}}`,
`{{server.uuid}}` and `{{config.docker.interface}}`.

### Variable rules

Laravel-style, separated by `|`: `required`, `nullable`, `string`, `numeric`,
`integer`, `boolean`, `min:n`, `max:n`, `between:a,b`, `in:a,b,c`,
`regex:/…/`, `alpha`, `alpha_num`, `alpha_dash`, `url`, `ip`. For numbers
`min`/`max` compare the value; for text, its length. `in:` rules show as a
dropdown in the panel and `boolean` as a switch.

## Pterodactyl eggs

Admins can import Pterodactyl/Pelican egg files (PTDL_v1 and v2) in the panel.
Config files, startup detection, the stop command and variable rules are
carried over. Variables that are clearly extra ports (`QUERY_PORT`,
`RCON_PORT`, …) become allocated ports.
