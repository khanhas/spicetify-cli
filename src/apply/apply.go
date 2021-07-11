package apply

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/khanhas/spicetify-cli/src/utils"
)

// Flag enables/disables additional feature
type Flag struct {
	Extension     []string
	CustomApp     []string
	SidebarConfig bool
	HomeConfig    bool
}

// AdditionalOptions .
func AdditionalOptions(appsFolderPath string, flags Flag) {
	filesToModified := map[string]func(path string, flags Flag){
		filepath.Join(appsFolderPath, "xpui", "index.html"):          htmlMod,
		filepath.Join(appsFolderPath, "xpui", "xpui.js"):             insertCustomApp,
		filepath.Join(appsFolderPath, "xpui", "xpui-routes-home.js"): insertHomeConfig,
	}

	for file, call := range filesToModified {
		if _, err := os.Stat(file); os.IsNotExist(err) {
			continue
		}

		call(file, flags)
	}

	if flags.SidebarConfig {
		utils.CopyFile(
			filepath.Join(utils.GetJsHelperDir(), "sidebarConfig.js"),
			filepath.Join(appsFolderPath, "xpui", "helper"))
	}

	if flags.HomeConfig {
		utils.CopyFile(
			filepath.Join(utils.GetJsHelperDir(), "homeConfig.js"),
			filepath.Join(appsFolderPath, "xpui", "helper"))
	}
}

// UserCSS creates user.css file in "xpui".
// To not use custom css, set `themeFolder` to blank string
// To use default color scheme, set `scheme` to `nil`
func UserCSS(appsFolderPath, themeFolder string, scheme map[string]string) {
	css := []byte(getColorCSS(scheme) + getUserCSS(themeFolder))

	dest := filepath.Join(appsFolderPath, "xpui", "user.css")
	if err := ioutil.WriteFile(dest, css, 0700); err != nil {
		utils.Fatal(err)
	}
}

// UserAsset .
func UserAsset(appsFolderPath, themeFolder string) {
	var assetsPath = getAssetsPath(themeFolder)
	var xpuiPath = filepath.Join(appsFolderPath, "xpui")
	if err := utils.Copy(assetsPath, xpuiPath, true, nil); err != nil {
		utils.Fatal(err)
	}
}

func htmlMod(htmlPath string, flags Flag) {
	if len(flags.Extension) == 0 &&
		!flags.HomeConfig &&
		!flags.SidebarConfig {
		return
	}

	extensionsHTML := "\n"
	helperHTML := "\n"

	if flags.SidebarConfig {
		helperHTML += `<script defer src="helper/sidebarConfig.js"></script>` + "\n"
	}

	if flags.HomeConfig {
		helperHTML += `<script defer src="helper/homeConfig.js"></script>` + "\n"
	}

	for _, v := range flags.Extension {
		if strings.HasSuffix(v, ".mjs") {
			extensionsHTML += `<script defer type="module" src="extensions/` + v + `"></script>` + "\n"
		} else {
			extensionsHTML += `<script defer src="extensions/` + v + `"></script>` + "\n"
		}
	}

	utils.ModifyFile(htmlPath, func(content string) string {
		utils.Replace(
			&content,
			`<\!-- spicetify helpers -->`,
			"${0}"+helperHTML)
		utils.Replace(
			&content,
			`</body>`,
			extensionsHTML+"${0}")
		return content
	})
}

func getUserCSS(themeFolder string) string {
	if len(themeFolder) == 0 {
		return ""
	}

	cssFilePath := filepath.Join(themeFolder, "user.css")
	_, err := os.Stat(cssFilePath)

	if err != nil {
		return ""
	}

	content, err := ioutil.ReadFile(cssFilePath)
	if err != nil {
		return ""
	}

	return string(content)
}

func getColorCSS(scheme map[string]string) string {
	var variableList string
	var variableRGBList string
	mergedScheme := make(map[string]string)

	for k, v := range scheme {
		mergedScheme[k] = v
	}

	for k, v := range utils.BaseColorList {
		if len(mergedScheme[k]) == 0 {
			mergedScheme[k] = v
		}
	}

	for k, v := range mergedScheme {
		parsed := utils.ParseColor(v)
		variableList += fmt.Sprintf("    --spice-%s: #%s;\n", k, parsed.Hex())
		variableRGBList += fmt.Sprintf("    --spice-rgb-%s: %s;\n", k, parsed.RGB())
	}

	return fmt.Sprintf(":root {\n%s\n%s\n}\n", variableList, variableRGBList)
}

func insertCustomApp(jsPath string, flags Flag) {
	utils.ModifyFile(jsPath, func(content string) string {
		reactSymbs := utils.FindSymbol(
			"Custom app React symbols",
			content,
			[]string{
				`lazy\(\(function\(\)\{return (\w+)\.(\w+)\(\d+\).then\(\w+\.bind\(\w+,\d+\)\)\}\)\)`})
		eleSymbs := utils.FindSymbol(
			"Custom app React Element",
			content,
			[]string{
				`createElement\(([\w\.]+),\{path:"\/collection"\}`})

		appMap := ""
		appReactMap := ""
		appEleMap := ""
		cssEnableMap := ""
		appNameArray := ""

		for index, app := range flags.CustomApp {
			appName := `spicetify-routes-` + app
			appMap += fmt.Sprintf(`"%s":"%s",`, appName, appName)
			appNameArray += fmt.Sprintf(`"%s",`, app)

			appReactMap += fmt.Sprintf(
				`,spicetifyApp%d=Spicetify.React.lazy((function(){return %s.%s("%s").then(%s.bind(%s,"%s"))}))`,
				index, reactSymbs[0], reactSymbs[1],
				appName, reactSymbs[0], reactSymbs[0], appName)

			appEleMap += fmt.Sprintf(
				`Spicetify.React.createElement(%s,{path:"/%s"},Spicetify.React.createElement(spicetifyApp%d,null)),`,
				eleSymbs[0], app, index)

			cssEnableMap += fmt.Sprintf(`,"%s":1`, appName)
		}

		utils.Replace(
			&content,
			`\{(\d+:"xpui)`,
			`{`+appMap+`${1}`)

		utils.ReplaceOnce(
			&content,
			`lazy\(\(function\(\)\{return [\w\.]+\(\d+\).then\(\w+\.bind\(\w+,\d+\)\)\}\)\)`,
			`${0}`+appReactMap)

		utils.ReplaceOnce(
			&content,
			`\w+\(\)\.createElement\([\w\.]+,\{path:"\/collection"\}`,
			appEleMap+`${0}`)

		utils.Replace(
			&content,
			`\w+\(\)\.createElement\("li",\{className:\w+\},\w+\(\)\.createElement\(\w+,\{uri:"spotify:user:@:collection",to:"/collection"\}`,
			`Spicetify._sidebarItemToClone=${0}`)

		utils.ReplaceOnce(
			&content,
			`\d+:1,\d+:1,\d+:1`,
			"${0}"+cssEnableMap)

		sidebarItemMatch := utils.SeekToCloseParen(
			content,
			`\("li",\{className:\w+\},\w+\(\)\.createElement\(\w+,\{uri:"spotify:user:@:collection",to:"/collection"\}`,
			'(', ')')

		content = strings.Replace(
			content,
			sidebarItemMatch,
			sidebarItemMatch+",Spicetify._cloneSidebarItem(["+appNameArray+"])",
			1)

		if flags.SidebarConfig {
			utils.ReplaceOnce(
				&content,
				`return null!=\w+&&\w+\.totalLength(\?\w+\(\)\.createElement\(\w+,\{contextUri:)(\w+)\.uri`,
				`return true${1}${2}?.uri||""`)
		}

		return content
	})
}

func insertHomeConfig(jsPath string, flags Flag) {
	if !flags.HomeConfig {
		return
	}

	utils.ModifyFile(jsPath, func(content string) string {
		utils.ReplaceOnce(
			&content,
			`(\w+\.filter\(\w+\))\.map`,
			`SpicetifyHomeConfig.arrange(${1}).map`)
		utils.ReplaceOnce(
			&content,
			`;(\(0,\w+\.useEffect\))`,
			`;${1}(()=>{SpicetifyHomeConfig.addToMenu();return SpicetifyHomeConfig.removeMenu;},[])${0}`)
		return content
	})
}

func getAssetsPath(themeFolder string) string {
	dir := filepath.Join(themeFolder, "assets")

	if _, err := os.Stat(dir); err != nil {
		return ""
	}

	return dir
}
