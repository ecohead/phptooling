package main

import (
	"embed"
	"fmt"
	"github.com/charmbracelet/huh"
	"gopkg.in/yaml.v2"
	"log"
	"os"
	"os/exec"
	"path"
	"sort"
	"strings"
)

var (
	docker                 bool
	dockerService          string
	tools                  []Tool
	toolsDirectory         = "./tools"
	preferredDockerCommand = "exec"
	composeServices        []string
	//go:embed all:config-files/*
	contentFS embed.FS
)

type Tool string

const (
	PhpCsFixer             Tool = "phpcsfixer"
	PhpStan                Tool = "phpstan"
	PhpCS                  Tool = "phpcs"
	PhpMD                  Tool = "phpmd"
	PhpCPD                 Tool = "phpcpd"
	ComposerRequireChecker Tool = "composer-require-checker"
)

type DirectoryType string

const (
	ParentDir DirectoryType = "parentDir"
	ToolDir   DirectoryType = "toolDir"
)

func main() {
	detectDockerConfiguration()
	servicesOptions := make([]huh.Option[string], len(composeServices))

	for i, service := range composeServices {
		servicesOptions[i] = huh.NewOption(service, service)
	}

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewConfirm().
				Title("Are you using docker in this project?").
				Affirmative("Yes").
				Negative("No").
				Value(&docker),
		),
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Which service do you want to use for running PHP commands?").
				Options(servicesOptions...).
				Value(&dockerService),
			huh.NewSelect[string]().
				Title("Which variant do you want to use for running commands?").
				Options(
					huh.NewOption("exec", "exec"),
					huh.NewOption("run", "run"),
				).
				Value(&preferredDockerCommand),
		).WithHideFunc(func() bool {
			return !docker
		}),
		huh.NewGroup(
			huh.NewInput().
				Title("In which directory tooling will be installed?").
				Placeholder("./tools").
				Value(&toolsDirectory),
			huh.NewMultiSelect[Tool]().
				Title("Which tools do you want to install?").
				Options(
					huh.NewOption("PHP CS Fixer", PhpCsFixer),
					huh.NewOption("PHPStan", PhpStan),
					huh.NewOption("PHP CS", PhpCS),
					huh.NewOption("PHP MD", PhpMD),
					huh.NewOption("PHP CPD", PhpCPD),
					huh.NewOption("Composer Require Checker", ComposerRequireChecker),
				).
				Value(&tools),
		),
	).WithTheme(huh.ThemeCatppuccin())

	err := form.Run()

	if err != nil {
		log.Fatal(err)
	}

	initializeJustFile()
	installTools()
	updateGitIgnore()
}

func detectDockerConfiguration() {
	composeFilePossibilities := []string{"docker-compose.yml", "docker-compose.yaml", "compose.yml", "compose.yaml"}
	composeFile := ""

	for _, file := range composeFilePossibilities {
		_, err := os.Stat(file)

		if err == nil {
			docker = true
			composeFile = file
			break
		}
	}

	if docker && composeFile != "" {
		composeServices = getComposeServices(composeFile)
	}
}

func getComposeServices(composeFile string) []string {
	m := make(map[interface{}]interface{})

	file, fileErr := os.ReadFile(path.Join(getLocalWorkingDirectory(), composeFile))

	if fileErr != nil {
		log.Fatal(fileErr)
	}

	parseErr := yaml.Unmarshal(file, &m)

	if parseErr != nil {
		log.Fatal(parseErr)
	}

	services := m["services"]
	var servicesList []string

	for name := range services.(map[interface{}]interface{}) {
		servicesList = append(servicesList, name.(string))
	}

	sort.Strings(servicesList)

	return servicesList
}

func getDockerCommandPrefix() []string {
	if preferredDockerCommand == "exec" {
		return []string{"compose", "exec", dockerService}
	} else {
		return []string{"compose", "run", "--rm", dockerService}
	}
}

func runCommand(command []string) {
	var cmd *exec.Cmd

	if docker {
		args := append(getDockerCommandPrefix(), command...)
		cmd = exec.Command("docker", args...)
	} else {
		cmd = exec.Command(command[0], command[1:]...)
	}

	fmt.Println("Running command: ", cmd.String())

	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout

	err := cmd.Run()

	if err != nil {
		log.Fatal(err)
	}
}

func getWorkingDirectory() string {
	if docker {
		workingDir, err := exec.Command("docker", append(getDockerCommandPrefix(), "pwd")...).Output()

		if err != nil {
			log.Fatal(err)
		}

		return strings.TrimSpace(string(workingDir))
	} else {
		return getLocalWorkingDirectory()
	}
}

func getLocalWorkingDirectory() string {
	workingDir, err := os.Getwd()

	if err != nil {
		log.Fatal(err)
	}

	return workingDir
}

func createDirectory(dirType DirectoryType, newPath string) string {
	var fullPath string

	if dirType == ParentDir {
		fullPath = path.Join(getWorkingDirectory(), newPath)
	} else if dirType == ToolDir {
		fullPath = path.Join(getToolsDirectory(), newPath)
	}

	runCommand([]string{"mkdir", "-p", fullPath})

	return fullPath
}

func getToolsDirectory() string {
	return path.Join(getWorkingDirectory(), toolsDirectory)
}

func installTools() {
	createDirectory(ParentDir, toolsDirectory)

	for _, tool := range tools {
		switch tool {
		case PhpCsFixer:
			installPhpCsFixer()
		case PhpStan:
			installPhpStan()
		case PhpCS:
			installPhpCS()
		case PhpMD:
			installPhpMD()
		case PhpCPD:
			installPhpCPD()
		case ComposerRequireChecker:
			installComposerRequireChecker()
		}
	}
}

func installComposerRequireChecker() {
	dir := createDirectory(ToolDir, "composer-require-checker")

	runCommand([]string{"composer", "require", "--dev", "maglnet/composer-require-checker", "--working-dir", dir})

	addToJustFile(func(composerAlias string, phpAlias string, toolsDir string) string {
		return `
# Launch Composer Require Checker (see https://github.com/maglnet/ComposerRequireChecker/)
check-deps:
	` + phpAlias + ` ` + toolsDir + `/composer-require-checker/vendor/bin/composer-require-checker check composer.json`
	})
}

func installPhpCPD() {
	dir := createDirectory(ToolDir, "phpcpd")

	runCommand([]string{"composer", "require", "--dev", "sebastian/phpcpd", "--working-dir", dir})

	addToJustFile(func(composerAlias string, phpAlias string, toolsDir string) string {
		return `
# Launch PHP Copy/Paste Detector (see https://github.com/sebastianbergmann/phpcpd)
phpcpd *paths='src/':
    ` + phpAlias + ` ` + toolsDir + `/phpcpd/vendor/bin/phpcpd {{paths}}
`
	})
}

func installPhpMD() {
	dir := createDirectory(ToolDir, "phpmd")

	runCommand([]string{"composer", "require", "--dev", "phpmd/phpmd", "--working-dir", dir})

	addToJustFile(func(composerAlias string, phpAlias string, toolsDir string) string {
		return `
# Launch PHP Mess Detector (see https://phpmd.org/)
phpmd *paths='src/':
    ` + phpAlias + ` ` + toolsDir + `/phpmd/vendor/bin/phpmd {{paths}} text .phpmd.xml
`
	})

	copyFile("config-files/phpmd/.phpmd.xml", path.Join(getWorkingDirectory(), ".phpmd.xml"))
}

func installPhpCS() {
	dir := createDirectory(ToolDir, "phpcs")

	runCommand([]string{"composer", "require", "--dev", "squizlabs/php_codesniffer", "escapestudios/symfony2-coding-standard", "--working-dir", dir})

	addToJustFile(func(composerAlias string, phpAlias string, toolsDir string) string {
		return `
# Launch PHP_CodeSniffer (see https://github.com/squizlabs/PHP_CodeSniffer)
phpcs:
    ` + phpAlias + ` ` + toolsDir + `/phpcs/vendor/bin/phpcs -s --standard=phpcs.xml.dist

# Launch PHP_CodeBeautifier (see https://github.com/squizlabs/PHP_CodeSniffer)
phpcbf *paths='./src ./tests':
    ` + phpAlias + ` ` + toolsDir + `/phpcs/vendor/bin/phpcbf --standard=phpcs.xml.dist {{paths}}
`
	})

	copyFile("config-files/phpcs/phpcs.xml.dist", path.Join(getWorkingDirectory(), "phpcs.xml.dist"))
}

func installPhpStan() {
	dir := createDirectory(ToolDir, "phpstan")

	runCommand([]string{"composer", "require", "--dev", "phpstan/phpstan", "phpstan/phpstan-symfony", "phpstan/phpstan-doctrine", "--working-dir", dir})

	addToJustFile(func(composerAlias string, phpAlias string, toolsDir string) string {
		return `
# Launch PHPStan (see https://phpstan.org/)
phpstan *paths='src':
    ` + phpAlias + ` ` + toolsDir + `/phpstan/vendor/bin/phpstan analyse -c phpstan.neon {{paths}}
`
	})

	copyFile("config-files/phpstan/phpstan.neon", path.Join(getWorkingDirectory(), "phpstan.neon"))
	copyFile("config-files/phpstan/console.php", path.Join(getWorkingDirectory(), "build", "console.php"))
	copyFile("config-files/phpstan/doctrine.php", path.Join(getWorkingDirectory(), "build", "doctrine.php"))
}

func installPhpCsFixer() {
	dir := createDirectory(ToolDir, "phpcsfixer")

	runCommand([]string{"composer", "require", "--dev", "friendsofphp/php-cs-fixer", "--working-dir", dir})

	addToJustFile(func(composerAlias string, phpAlias string, toolsDir string) string {
		return `
# Launch PHP CS Fixer (see https://github.com/PHP-CS-Fixer/PHP-CS-Fixer)
phpcsfixer:
    ` + phpAlias + ` ` + toolsDir + `/php-cs-fixer/vendor/bin/php-cs-fixer fix
`
	})

	copyFile("config-files/phpcsfixer/.php-cs-fixer.dist.php", path.Join(getWorkingDirectory(), ".php-cs-fixer.dist.php"))
}

type justFileCallback func(composerAlias string, phpAlias string, toolsDir string) string

func addToJustFile(callback justFileCallback) {
	file, fileErr := os.OpenFile("justfile", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)

	if fileErr != nil {
		log.Fatal(fileErr)
	}

	var composerAlias string
	var phpAlias string

	if docker {
		composerAlias = "docker " + strings.Join(getDockerCommandPrefix(), " ") + " composer"
		phpAlias = "docker " + strings.Join(getDockerCommandPrefix(), " ") + " php"
	} else {
		composerAlias = "composer"
		phpAlias = "php"
	}

	toolsDir := getToolsDirectory()

	_, writeErr := file.WriteString(callback(composerAlias, phpAlias, toolsDir))

	if writeErr != nil {
		log.Fatal(writeErr)
	}

	closeErr := file.Close()

	if closeErr != nil {
		log.Fatal(closeErr)
	}
}

func initializeJustFile() {
	addToJustFile(func(composerAlias string, phpAlias string, toolsDir string) string {
		return `
# Install php dependencies
install-php:
    ` + composerAlias + ` install
    ` + composerAlias + ` install --working-dir=` + toolsDir + `/phpcs
    ` + composerAlias + ` install --working-dir=` + toolsDir + `/phpmd
    ` + composerAlias + ` install --working-dir=` + toolsDir + `/phpcsfixer
    ` + composerAlias + ` install --working-dir=` + toolsDir + `/phpstan
    ` + composerAlias + ` install --working-dir=` + toolsDir + `/phpcpd
    ` + composerAlias + ` install --working-dir=` + toolsDir + `/composer-require-checker
`
	})
}

func updateGitIgnore() {
	file, fileErr := os.OpenFile(".gitignore", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)

	if fileErr != nil {
		log.Fatal(fileErr)
	}

	_, writeErr := file.WriteString(`
###> php-tooling ###
.DS_Store
.php-cs-fixer.cache
.phpcs.cache
.idea/
.vscode/
vendor/
###< php-tooling ###`)

	if writeErr != nil {
		log.Fatal(writeErr)
	}

	closeErr := file.Close()

	if closeErr != nil {
		log.Fatal(closeErr)
	}
}

/**
 * Copy file from the config-files directory to destination
 */
func copyFile(filePath string, destination string) {
	data, err := contentFS.ReadFile(filePath)
	if err != nil {
		log.Fatal(err)
	}

	fileDir := path.Dir(destination)
	// Create directory if it doesn't exist
	runCommand([]string{"mkdir", "-p", fileDir})
	// Create file with 644 permissions to avoid issues with other tools or IDE
	runCommand([]string{"touch", destination})
	runCommand([]string{"chmod", "644", destination})
	// Using bash to avoid escaping issues, quotes around EOL are necessary to avoid variable expansion
	runCommand([]string{"bash", "-c", "cat > " + destination + " <<'EOL'\n" + string(data) + "\nEOL"})
}
