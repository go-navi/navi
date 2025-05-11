# Navi - Lightweight Command Runner

<p align="center">
  <img src=".github/navi.svg" alt="Navi Logo" width="80"/>
</p>

A lightweight, cross-platform command runner tool that simplifies development workflows across multiple projects, languages, and frameworks.

> **Note**: Navi was created as an AI-powered software development experiment, with most of the code (including this documentation) generated through AI assistance.

## Overview

Navi helps developers organize and execute commands from a single configuration file, providing an intuitive way to manage complex project workflows through a simple interface.

### Key Features

- 🖥️ **Interactive CLI** - Navigate and run commands through a user-friendly terminal interface
- 📂 **Project Organization** - Define and group commands by project
- 🔄 **Command Runners** - Create sequence runners to execute multiple commands in series or parallel
- 👀 **File Watching** - Automatically reload commands when files change
- ⏳ **Port Awaiting** - Wait for services to be ready for connection on specific ports
- 🔁 **Auto-Restart** - Configure auto-restart behaviors with custom retry settings
- 🔒 **Environment Variables** - Handle environment variables with selective loading from .env files
- 🪝 **Command Hooks** - Execute pre/post hooks and conditional after-commands
- 🐚 **Shell Support**: Execute commands with the shell of your choice (bash, zsh, powershell, cmd, etc.)
- 💻 **Cross-Platform** - Works seamlessly on Windows, macOS, and Linux

## Installation

### Download Binary

Download the latest binary for your platform from the [releases page](https://github.com/go-navi/navi/releases) and add it to your system's `PATH`.

### Install via NPM

```bash
sudo npm install -g go-navi
```

### Build from Source

```bash
git clone https://github.com/go-navi/navi.git
cd navi
go build -ldflags="-s -w" -trimpath
```

## Quick Start

1. Create a `navi.yml` file in your project root:

```yaml
commands:
  lint: eslint . --ext .js,.ts
  docker-up:
    - cd container
    - docker-compose up -d

projects:
  api:
    dir: ./backend
    cmds:
      dev: go run main.go
      test: go test .

  web:
    dir: ./frontend
    cmds:
      dev: npm run dev
      test: npm run test
      install: npm install

runners:
  start-all:
    - api:dev
    - web:dev

  test-all[serial]:
    - docker-up
    - api:test
    - web:test
```

---

2. Run `navi` in your terminal to launch the interactive CLI:

```bash
navi
```

![Navi Interactive CLI](.github/cli1.png "Navi Interactive CLI")

---

3. Or execute commands directly:

```bash
navi docker-up             # Run `docker-up` single command
navi web:dev               # Run 'dev' command of the 'web' project
navi web:install express   # Run project command passing `express` argument
navi api go build -v       # Run 'go build' on the 'api' project folder
navi api:*                 # Run all commands of the 'api' project
navi start-all             # Run `start-all` runner
navi test-all              # Run `test-all` serial runner
navi lint web:dev api:dev  # Run multiple commands or project commands
```

## Command Configuration

Define commands with various formats:

```yaml
commands:
  # Simple command
  simple: docker-compose up -d

  # Sequential multi-command
  multi:
    - echo "Setting up environment..."
    - python3 setup.py

  # Detailed configuration
  complex:
    dir: ./app                    # Command's directory path
    run: python3 server.py        # Main command
    shell: zsh                    # Override default system's shell
    watch: ["src/**/*.py"]        # Watch file patterns
    dotenv: .env.local            # Load environment variables
    env:                          # Additional environment variables
      API_URL: localhost:${PORT}  # Use system's env variable (PORT)
      RATE_LIMIT: 50
      DEBUG: true
    pre: echo "Starting..."       # Run before main command
    post: echo "Stopped!"         # Run after main command
    after:                        # Executed based on command result
      success: echo "Success!"
      failure: echo "Failed!"
      always: python3 clean-up.py
```

## Project Configuration

Group commands with shared settings:

```yaml
projects:
  backend:
    dir: ./api                      # Working project directory (required)
    shell: zsh                      # Shared shell
    env: { NODE_ENV: development }  # Shared environment variables
    dotenv: .env.api                # Shared environment file
    pre: npm run pre-proj           # Commands to run before all commands
    post: npm run post-proj         # Commands to run after all commands
    after: node clear-resources.js  # After hook to be executed after all commands
    watch:                          # Shared watch patterns
      include: ["src/**/*.js"]
      exclude: ["**/tests/**"]
    cmds:                           # Project commands (required)
      start: node server.js
      test:
        dir: ./tests                # Subdirectory (./api/tests)
        env: { NODE_ENV: test }     # Override project settings
        run:                        # Command(s) to execute (required)
         - jest .
         - node export-logs.js
```

## Runner Configuration

Execute multiple commands or project commands:

```yaml
runners:
  # Simple list of commands
  dev:
    - "backend:start"
    - "frontend:dev"

  # Advanced configuration
  full-stack:
    - docker-up             # Execute simple command

    - cmd: db:start
      name: "Database"      # Display name
      delay: 2              # Start delay in seconds
      awaits: 3000          # Wait for port to be ready for connection
      restart: true         # Auto-restart on failure
      serial: true          # Next commands wait for this to finish

    - cmd: backend:start
      dependent: true       # All commands will stop if this fails
      awaits:
        timeout: 15         # Timeout in seconds (default = 30)
        ports:              # Wait for API ports to be ready for connection
          - 3001
          - 5432
      restart:
        retries: 3          # Maximum restart attempts (default = infinite)
        interval: 5         # Seconds between retries (default = 1)
        condition: failure  # Restart condition: success, failure (default), always

    - frontend:*            # Execute all commands of a project
```

## Command Line Options

```
Usage: navi [options] [commands...]

Options:
  -f, --file <path>     Specify config file (default: ./navi.yml)
  -s, --serial          Execute runner commands serially
  -d, --dependent       Make runner commands dependent
  -h, --help            Show help information
  -v, --version         Show current version
```

## Advanced Configuration

### Detailed Properties

Some command properties can also be defined in a detailed format. They will either inherit or override parent's command or project properties.

```yaml
commands:
  # Detailed properties
  complex:
    dir: ./server
    shell: zsh
    env: { DEBUG: false }
    dotenv: .env
    run: python3 server.py -p ${PORT}

    # Detailed `pre`
    pre:
      dir: ./utils                  # Path relative to parent's `dir` (./server/utils)
      run: python3 init_logger.py   # Executes ./server/utils/init_logger.py

    # Detailed `post`
    post:
      dir: __ROOT__/tests           # Path relative to root folder
      env: { DEBUG: true }          # Override parent DEBUG value
      dotenv: .env.tests            # Loads `.env.tests` along with root `.env` file
      run: python3 main_test.py     # Executes ./tests/main_test.py

    # Detailed `after`
    after:                          # Shorter for `after.always`
      shell: csh                    # Override parent shell
      run: python3 shutdown.py      # Execute shutdown.py on root folder

    # Detailed `watch`
    watch:
      include:                      # file patterns to watch
        - "./src/**/*.py"
        - "./utils/config.json"
      exclude:                      # file patterns to ignore
        - "**/tests/**"
```

#### Tips:

- You can reference the system's environment variables in configuration settings by using `${ENV_KEY}` format.

- You can also use `__ROOT__` to refer to the directory of the `navi.yml` file.

- If no shell is defined, the default shell will be used on macOS and Linux (usually `bash` on Linux and `zsh` on macOS). `cmd` will be used by default on Windows.

- The `after` command, unlike `post`, can be executed even if the main command fails, making it ideal for cleanup or graceful shutdown tasks. If you want to ensure a command will run after the main command, prefer using `after` or `after.always` (longer version).

- `run`, `pre`, `post`, `after`, `after.success`, `after.failure` and `after.always` are also considered to be commands, and can all be written in the format of a detailed command.

- `run`, `dotenv`, `pre`, `post`, `after`, `after.success`, `after.failure`, `after.always`, `watch`, `watch.include` and `watch.exclude` can all be written in the format of a simple string or an array of strings.

### Runner Flags

By default, runner commands execute in **parallel** and are **not dependent** of each other (if one fails, others won't stop). You can change this behavior by adding flags to the runner.

```yaml
runners:
  # Make all commands run sequentially and dependently
  test[serial,dependent]:
    - db:init
    - cmd: backend:test
      awaits: 5432
```

In fact, there are multiple ways to achieve this result:

- Defining flags in the `yaml` config: _`[serial]` or `[dependent]`_

- Executing Navi passing the flag arguments: _`-s`,`--serial`_ or _`-d`,`--dependent`_

- Using `serial` or `dependent` settings for individual runner commands in the `yaml` config.

### Interactive CLI Comments

You can add comments on top of commands, project commands and runners to describe them in the Interactive CLI tool.

```yaml
commands:
  # Apply linting rules to JS/TS files
  lint: eslint . --ext .js,.ts

projects:
  server:
    dir: ./api
    cmds:
      # Build the backend of the application
      build: npm run build

runners:
  # Set up and build the application
  build-project[serial]:
    - lint
    - server:build
```

![Navi Interactive CLI](.github/cli2.png "Navi Interactive CLI")

## License

[MIT](LICENSE)

---

Navi is designed to assist with local development workflows. It is not intended to replace production-grade process managers (like `PM2`).
