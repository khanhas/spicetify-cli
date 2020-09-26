package cmd

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/go-ini/ini"
	"github.com/khanhas/spicetify-cli/src/utils"
)

var (
	spicetifyFolder         = getSpicetifyFolder()
	rawFolder, themedFolder = getExtractFolder()
	backupFolder            = getUserFolder("Backup")
	userThemesFolder        = getUserFolder("Themes")
	userExtensionsFolder    = getUserFolder("Extensions")
	userAppsFolder          = getUserFolder("CustomApps")
	quiet                   bool
	isAppX                  = false
	spotifyPath             string
	prefsPath               string
	appPath                 string
	appDestPath             string
	cfg                     utils.Config
	settingSection          *ini.Section
	backupSection           *ini.Section
	preprocSection          *ini.Section
	featureSection          *ini.Section
	themeFolder             string
	colorCfg                *ini.File
	colorSection            *ini.Section
	injectCSS               bool
	replaceColors           bool
	overwriteAssets         bool
)

// InitConfig gets and parses config file.
func InitConfig(isQuiet bool) {
	quiet = isQuiet

	cfg = utils.ParseConfig(GetConfigPath())
	settingSection = cfg.GetSection("Setting")
	backupSection = cfg.GetSection("Backup")
	preprocSection = cfg.GetSection("Preprocesses")
	featureSection = cfg.GetSection("AdditionalOptions")
}

// InitPaths checks various essential paths' availablities,
// tries to auto-detect them and stops spicetify when any one
// of them is invalid.
func InitPaths() {
	spotifyPath = settingSection.Key("spotify_path").String()

	if len(spotifyPath) != 0 {
		if _, err := os.Stat(spotifyPath); err != nil {
			utils.PrintError(spotifyPath + ` does not exist or is not a valid path. Please manually set "spotify_path" in config.ini to correct directory of Spotify.`)
			os.Exit(1)
		}
	} else if spotifyPath = utils.FindAppPath(); len(spotifyPath) != 0 {
		settingSection.Key("spotify_path").SetValue(spotifyPath)
		cfg.Write()
	} else {
		utils.PrintError(`Cannot detect Spotify location. Please manually set "spotify_path" in config.ini`)
		os.Exit(1)
	}

	prefsPath = settingSection.Key("prefs_path").String()

	if len(prefsPath) != 0 {
		if _, err := os.Stat(prefsPath); err != nil {
			utils.PrintError(prefsPath + ` does not exist or is not a valid path. Please manually set "prefs_path" in config.ini to correct path of "prefs" file.`)
			os.Exit(1)
		}
	} else if prefsPath = utils.FindPrefFilePath(); len(prefsPath) != 0 {
		settingSection.Key("prefs_path").SetValue(prefsPath)
		cfg.Write()
	} else {
		utils.PrintError(`Cannot detect Spotify "prefs" file location. Please manually set "prefs_path" in config.ini`)
		os.Exit(1)
	}

	if runtime.GOOS == "windows" {
		isAppX = strings.Contains(spotifyPath, "SpotifyAB.SpotifyMusic")
	}

	appPath = filepath.Join(spotifyPath, "Apps")

	if isAppX {
		appDestPath = filepath.Join(spicetifyFolder, "AppX")
	} else {
		appDestPath = appPath
	}

	utils.CheckExistAndCreate(appDestPath)
}

// InitSetting parses theme settings and gets color section.
func InitSetting() {
	replaceColors = settingSection.Key("replace_colors").MustBool(false)
	injectCSS = settingSection.Key("inject_css").MustBool(false)
	overwriteAssets = settingSection.Key("overwrite_assets").MustBool(false)

	themeName := settingSection.Key("current_theme").String()

	if len(themeName) == 0 {
		injectCSS = false
		replaceColors = false
		overwriteAssets = false
		return
	}

	themeFolder = getThemeFolder(themeName)

	colorPath := filepath.Join(themeFolder, "color.ini")
	cssPath := filepath.Join(themeFolder, "user.css")
	assetsPath := filepath.Join(themeFolder, "assets")

	if replaceColors {
		_, err := os.Stat(colorPath)
		replaceColors = err == nil
	}

	if injectCSS {
		_, err := os.Stat(cssPath)
		injectCSS = err == nil
	}

	if overwriteAssets {
		_, err := os.Stat(assetsPath)
		overwriteAssets = err == nil
	}

	if !replaceColors {
		return
	}

	var err error
	colorCfg, err = ini.InsensitiveLoad(colorPath)
	if err != nil {
		utils.PrintError("Cannot open file " + colorPath)
		replaceColors = false
		return
	}

	sections := colorCfg.Sections()

	if len(sections) < 2 {
		utils.PrintError("No section found in " + colorPath)
		replaceColors = false
		return
	}

	schemeName := settingSection.Key("color_scheme").String()
	if len(schemeName) == 0 {
		colorSection = sections[1]
		return
	}

	schemeSection, err := colorCfg.GetSection(schemeName)
	if err != nil {
		colorSection = sections[1]
		return
	}

	colorSection = schemeSection
}

// GetConfigPath returns location of config file
func GetConfigPath() string {
	return filepath.Join(spicetifyFolder, "config.ini")
}

// GetSpotifyPath returns location of Spotify client
func GetSpotifyPath() string {
	return spotifyPath
}

func getSpicetifyFolder() string {
	result, isAvailable := os.LookupEnv("SPICETIFY_CONFIG")
	defer func() { utils.CheckExistAndCreate(result) }()

	if isAvailable && len(result) > 0 {
		return result
	}

	if runtime.GOOS == "windows" {
		result = filepath.Join(os.Getenv("USERPROFILE"), ".spicetify")

	} else if runtime.GOOS == "linux" || runtime.GOOS == "darwin" {
		parent, isAvailable := os.LookupEnv("XDG_CONFIG_HOME")

		if !isAvailable || len(parent) == 0 {
			parent = filepath.Join(os.Getenv("HOME"), ".config")
		}

		result = filepath.Join(parent, "spicetify")

	}
	return result
}

// getUserFolder checks if folder `name` is available in spicetifyFolder,
// else creates then returns the path.
func getUserFolder(name string) string {
	dir := filepath.Join(spicetifyFolder, name)
	utils.CheckExistAndCreate(dir)

	return dir
}

func getExtractFolder() (string, string) {
	dir := getUserFolder("Extracted")

	raw := filepath.Join(dir, "Raw")
	utils.CheckExistAndCreate(raw)

	themed := filepath.Join(dir, "Themed")
	utils.CheckExistAndCreate(themed)

	return raw, themed
}

func getThemeFolder(themeName string) string {
	folder := filepath.Join(userThemesFolder, themeName)
	_, err := os.Stat(folder)
	if err == nil {
		return folder
	}

	folder = filepath.Join(utils.GetExecutableDir(), "Themes", themeName)
	_, err = os.Stat(folder)
	if err == nil {
		return folder
	}

	utils.PrintError(`Theme "` + themeName + `" not found`)
	os.Exit(1)
	return ""
}

// ReadAnswer prints out a yes/no form with string from `info`
// and returns boolean value based on user input (y/Y or n/N) or
// return `defaultAnswer` if input is omitted.
// If input is neither of them, print form again.
// If app is in quiet mode, returns quietModeAnswer without promting.
func ReadAnswer(info string, defaultAnswer bool, quietModeAnswer bool) bool {
	if quiet {
		return quietModeAnswer
	}

	reader := bufio.NewReader(os.Stdin)
	fmt.Print(info)
	text, _ := reader.ReadString('\n')
	text = strings.Replace(text, "\r", "", 1)
	text = strings.Replace(text, "\n", "", 1)
	if len(text) == 0 {
		return defaultAnswer
	} else if text == "y" || text == "Y" {
		return true
	} else if text == "n" || text == "N" {
		return false
	}
	return ReadAnswer(info, defaultAnswer, quietModeAnswer)
}

// CheckUpgrade fetchs latest package version from Github API and inform user if there is new release
func CheckUpgrade(version string) {
	if !settingSection.Key("check_spicetify_upgrade").MustBool() {
		return
	}

	latestTag := FetchLatestTag()
	if latestTag == version {
		utils.PrintInfo("spicetify up-to-date")
	} else {
		utils.PrintWarning("New version available!")
		utils.PrintWarning(`Run "spicetify upgrade" or using package manager to upgrade spicetify`)
	}
}
