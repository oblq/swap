package swap

import (
	"embed"
	"fmt"
	"io/fs"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"text/template"
)

// FileSearchCaseSensitive determine config files search mode, false by default.
var FileSearchCaseSensitive bool

type FileSystem interface {
	ConfigPath() string
	WalkConfigPath(
		configPath string,
		regex *regexp.Regexp,
	) (matchedFile string, err error)
	ReadFile(filePath string) (file []byte, err error)
	ParseTemplate(templatePath string) (*template.Template, error)
}

//----------------------------------------------------------------------------------------------------------------------

type fileSystemLocal struct {
	configPath string
}

func NewFileSystemLocal(configPath string) FileSystem {
	return &fileSystemLocal{configPath: configPath}
}

func (fsl *fileSystemLocal) ConfigPath() string {
	return fsl.configPath
}

// WalkConfigPath look for a file matching the passed regex skipping sub-directories.
func (fsl *fileSystemLocal) WalkConfigPath(
	configPath string,
	regex *regexp.Regexp,
) (matchedFile string, err error) {
	err = filepath.Walk(configPath, func(path string, info os.FileInfo, err error) error {
		// nil if the path does not exist
		if info == nil {
			return filepath.SkipDir
		}

		if info.IsDir() && info.Name() != filepath.Base(configPath) {
			return filepath.SkipDir
		}

		if !info.Mode().IsRegular() {
			return nil
		}

		if regex.MatchString(info.Name()) {
			matchedFile = path
		}

		return nil
	})

	return
}

func (fsl *fileSystemLocal) ReadFile(filePath string) (file []byte, err error) {
	return ioutil.ReadFile(filePath)
}

func (fsl *fileSystemLocal) ParseTemplate(templatePath string) (*template.Template, error) {
	return template.ParseFiles(templatePath)
}

//----------------------------------------------------------------------------------------------------------------------

type fileSystemEmbedded struct {
	fs         embed.FS
	configPath string
}

func NewFileSystemEmbedded(fs embed.FS, configPath string) FileSystem {
	return &fileSystemEmbedded{fs, configPath}
}

func (fse *fileSystemEmbedded) ConfigPath() string {
	return fse.configPath
}

// WalkConfigPath look for a file matching the passed regex skipping sub-directories.
func (fse *fileSystemEmbedded) WalkConfigPath(configPath string, regex *regexp.Regexp) (matchedFile string, err error) {
	configPath = strings.Trim(configPath, string(filepath.Separator))
	err = fs.WalkDir(fse.fs, configPath, func(path string, d fs.DirEntry, err error) error {
		if d == nil {
			return nil
		}

		// nil if the path does not exist or if that is a dir and not a file
		info, err := d.Info()
		if info == nil || err != nil {
			return filepath.SkipDir
		}

		if info.IsDir() && info.Name() != filepath.Base(configPath) {
			return nil
		}

		if !info.Mode().IsRegular() {
			return nil
		}

		if regex.MatchString(info.Name()) {
			matchedFile = path
		}

		return nil
	})

	return
}

func (fse *fileSystemEmbedded) ReadFile(filePath string) (file []byte, err error) {
	return fse.fs.ReadFile(filePath)
}

func (fse *fileSystemEmbedded) ParseTemplate(templatePath string) (*template.Template, error) {
	return template.ParseFS(fse.fs, templatePath)
}

//----------------------------------------------------------------------------------------------------------------------

func getConfigPathsByFieldTagFileNames(
	configFileNamesFromTags []string,
	fs FileSystem,
	env *Environment,
) (completeFilePaths []string, err error) {
	for i, file := range configFileNamesFromTags {
		configFileNamesFromTags[i] = filepath.Join(fs.ConfigPath(), file)
	}

	return appendEnvFiles(configFileNamesFromTags, fs, env)
}

// appendEnvFiles will search for the given file names in the given path
// returning all the eligible files (e.g.: <path>/config.yaml or <path>/config.<environment>.json)
//
// If fs is nil a new local FileSystem will be automatically used with the
// configPath set as the one of any file scanned.
//
// Files name can also be passed without file extension,
// configFilesByEnv is semi-agnostic and will match any
// supported extension using the regex: `(?i)(.y(|a)ml|.toml|.json)`.
//
// The 'file' name will be searched as (in that order):
//  - '<path>/<file>(.* || <the_provided_extension>)'
//  - '<path>/<file>.<environment>(.* || <the_provided_extension>)'
//
// The latest found files will override previous.
func appendEnvFiles(
	completeFilePaths []string,
	fs FileSystem,
	env *Environment,
) (completeFilePathsPlusEnvFiles []string, err error) {
	for _, file := range completeFilePaths {
		configPath, fileName := filepath.Split(file)
		if len(configPath) == 0 {
			configPath = "."
		}

		if fs == nil {
			fs = globalFS
		}

		ext := filepath.Ext(fileName)
		extTrimmed := strings.TrimSuffix(fileName, ext)
		if len(ext) == 0 {
			ext = regexpValidExt.String() // search for any compatible file
		}

		format := "^%s%s$"
		if !FileSearchCaseSensitive {
			format = "(?i)(^%s)%s$"
		}
		// look for the config file in the config path (e.g.: tool.yml)
		regex := regexp.MustCompile(fmt.Sprintf(format, extTrimmed, ext))
		var foundFile string
		foundFile, err = fs.WalkConfigPath(configPath, regex)
		if err != nil {
			break
		}
		if len(foundFile) > 0 {
			completeFilePathsPlusEnvFiles = append(completeFilePathsPlusEnvFiles, foundFile)
		}

		if env != nil {
			// look for the env config file in the config path (eg.: tool.development.yml)
			//regexEnv := regexp.MustCompile(fmt.Sprintf(format, fmt.Sprintf("%s.%s", extTrimmed, Env().ID()), ext))
			regexEnv := regexp.MustCompile(fmt.Sprintf(format, fmt.Sprintf("%s.%s", extTrimmed, env.Tag()), ext))
			foundFile, err = fs.WalkConfigPath(configPath, regexEnv)
			if err != nil {
				break
			}
			if len(foundFile) > 0 {
				completeFilePathsPlusEnvFiles = append(completeFilePathsPlusEnvFiles, foundFile)
			}
		}
	}

	if err == nil && len(completeFilePathsPlusEnvFiles) == 0 {
		err = fmt.Errorf("no config file found for '%s'", strings.Join(completeFilePaths, " | "))
	}
	return
}
