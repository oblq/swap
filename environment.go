package swap

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	re "regexp"
	"strings"
	"sync"

	"github.com/oblq/swap/internal/logger"
)

// InterpolableEnvTag define the current environment.
// Can be defined by code or, since it is an exported string,
// can be interpolated with -ldflags at build/run time:
// 	go build -ldflags "-X github.com/oblq/swap.InterpolableEnvTag=develop" -v -o ./api_bin ./api
//var InterpolableEnvTag string

var testingRegexp = re.MustCompile(`_test|(\.test$)|_Test`)

//----------------------------------------------------------------------------------------------------------------------

// Environment struct represent an arbitrary environment with its
// tag and regexp (to detect it based on custom criterions).
type Environment struct {
	// tag is the primary environment tag and
	// the part of the config files name that the config parser
	// will look for while searching for environment specific files.
	tag string

	// regexp for environment tags.
	regexp *re.Regexp

	// inferredBy remember from where the buildEnvironment has been determined.
	inferredBy string
}

// NewEnvironment create a new instance of Environment.
// It will panic if an invalid regexp is provided or
// if the regexp does not match the primary tag.
//
// `tag` is the primary environment tag and
// the part of the config files name that the config parser
// will look for while searching for environment specific files.
//
// `regexp` must match all the tags we consider valid for the receiver.
func NewEnvironment(tag, regexp string) *Environment {
	e := &Environment{tag: tag, regexp: re.MustCompile(regexp)}
	if !e.regexp.MatchString(tag) {
		panic(fmt.Errorf("the environment Tag must be matched by its regexp. Tag: %s, regexp: %s",
			e.Tag(), e.regexp.String()))
	}
	return e
}

// Tag return the primary tag of the receiver.
func (e *Environment) Tag() string {
	return e.tag
}

// MatchTag return true if the environment regexp
// match the passed string.
func (e *Environment) MatchTag(tag string) bool {
	return e.regexp.MatchString(tag)
}

// Info returns some environment info.
func (e *Environment) Info() string {
	return fmt.Sprintf("Environment: %s. Tag: %s\n", logger.Green(strings.ToUpper(e.Tag())), e.inferredBy)
}

//----------------------------------------------------------------------------------------------------------------------

type defaultEnvs struct {
	Production  *Environment
	Staging     *Environment
	Testing     *Environment
	Development *Environment
	Local       *Environment
}

func (de defaultEnvs) Slice() []*Environment {
	return []*Environment{
		DefaultEnvs.Production,
		DefaultEnvs.Staging,
		DefaultEnvs.Testing,
		DefaultEnvs.Development,
		DefaultEnvs.Local,
	}
}

// Default environment's configurations.
var DefaultEnvs = defaultEnvs{
	Production:  NewEnvironment("production", `(production)|(master)|(^v(0|[1-9]+)(\\.(0|[1-9]+)+)?(\\.(\\*|(0|[1-9]+)+))?$)`),
	Staging:     NewEnvironment("staging", `(staging)|(release/*)|(hotfix/*)|(bugfix/*)`),
	Testing:     NewEnvironment("testing", `(testing)|(test)`),
	Development: NewEnvironment("development", `(development)|(develop)|(dev)|(feature/*)`),
	Local:       NewEnvironment("local", `local`),
}

//----------------------------------------------------------------------------------------------------------------------

type Sources struct {
	// directEnvironmentTag can be used to directly define the current environment.
	// By default, the value of `InterpolableEnvTag` is set.
	// Leave empty if you don't need to override the environment manually.
	directEnvironmentTag string

	// SystemEnvironmentTagKey is the system environment variable key
	// for the build environment tag, the default value is 'BUILD_ENV'.
	SystemEnvironmentTagKey string

	// Git is the project version control system.
	// The default path is './' (the working directory).
	Git *Repository
}

type EnvironmentHandler struct {
	// Sources define the sources used to determine the current environment.
	Sources *Sources

	currentTAG string

	environments []*Environment
	// any other custom environment can be added later.
	// by default, it includes the five standard ones and
	// environments hold all the environments to check,
	// determine the current environment.
	// currentTAG is the tag from which environmentsHandler

	mutex sync.Mutex
}

// NewEnvironmentHandler return a new instance of environmentHandler
// with default Sources and the passed environments.
//
// Sources define the sources used to determine the current environment.
// If DirectEnvironmentTag is empty then the
// system environment variable SystemEnvironmentTagKey will be checked,
// if also the system environment variable is empty the Git.BranchName will be used.
func NewEnvironmentHandler(environments []*Environment) *EnvironmentHandler {
	return &EnvironmentHandler{
		Sources: &Sources{
			//directEnvironmentTag:    InterpolableEnvTag,
			SystemEnvironmentTagKey: "BUILD_ENV",
			Git:                     NewGitRepository("./"),
		},
		environments: environments,
	}
}

// SetCurrent set the current environment using a tag.
// It must be matched by one of the environments regexp.
func (eh *EnvironmentHandler) SetCurrent(tag string) {
	eh.Sources.directEnvironmentTag = tag
}

// Current returns the current active environment by
// matching the found tag against any environments regexp.
func (eh *EnvironmentHandler) Current() *Environment {
	eh.mutex.Lock()
	defer eh.mutex.Unlock()

	inferredBy := ""

	if len(eh.Sources.directEnvironmentTag) > 0 {
		eh.currentTAG = eh.Sources.directEnvironmentTag
		inferredBy = fmt.Sprintf("'%s', from `SetCurrent()`, set manually.", eh.currentTAG)
	} else if eh.currentTAG = os.Getenv(eh.Sources.SystemEnvironmentTagKey); len(eh.currentTAG) > 0 {
		inferredBy = fmt.Sprintf("'%s', from `%s` environment variable.",
			eh.currentTAG, eh.Sources.SystemEnvironmentTagKey)
	} else if eh.Sources.Git != nil {
		if eh.Sources.Git.Error == nil {
			eh.currentTAG = eh.Sources.Git.BranchName
			inferredBy = fmt.Sprintf("<empty>, from git.BranchName (%s).", eh.Sources.Git.BranchName)
		}
	} else if testingRegexp.MatchString(os.Args[0]) {
		eh.currentTAG = DefaultEnvs.Testing.Tag()
		inferredBy = fmt.Sprintf("`%s`, from the running file name (%s).", eh.currentTAG, os.Args[0])
	} else {
		inferredBy = "<empty>, default environment is `local`."
	}

	env := DefaultEnvs.Local
	env.inferredBy = inferredBy

	for _, e := range eh.environments {
		if e.MatchTag(eh.currentTAG) {
			e.inferredBy = inferredBy
			env = e
			break
		}
	}

	return env
}

// Git -----------------------------------------------------------------------------------------------------------------

// gitRepository represent a git repository
type Repository struct {
	path                           string
	BranchName, Commit, Build, Tag string

	Error error
	mutex sync.Mutex
}

// NewRepository return a new gitRepository instance for the given path
func NewGitRepository(path string) *Repository {
	repo := &Repository{path: path}
	defer repo.updateInfo()
	return repo
}

// Info return Git repository info.
func (g *Repository) Info() string {
	g.mutex.Lock()
	defer g.mutex.Unlock()

	gitLog := logger.KVLogger{ValuePainter: logger.Magenta}
	return fmt.Sprintf("%s\n%s\n%s\n%s\n",
		gitLog.Sprint("Git Branch:", g.BranchName),
		gitLog.Sprint("Git Commit:", g.Commit),
		gitLog.Sprint("Git Tag:", g.Tag),
		gitLog.Sprint("Git Build:", g.Build))
}

// updateInfo grab git info and set 'Error' var eventually.
func (g *Repository) updateInfo() {
	g.mutex.Lock()
	defer g.mutex.Unlock()

	g.BranchName = g.git("rev-parse", "--abbrev-ref", "HEAD")
	g.Commit = g.git("rev-parse", "--short", "HEAD")
	g.Build = g.git("rev-list", "--all", "--count")
	g.Tag = g.git("describe", "--abbrev=0", "--tags", "--always")
}

// Git is the bash git command.
func (g *Repository) git(params ...string) string {
	cmd := exec.Command("git", params...)
	if len(g.path) > 0 {
		cmd.Dir = g.path
	}

	output, err := cmd.Output()
	if err != nil {
		gitErrString := err.Error()
		// not a repository error...
		if exitError, ok := err.(*exec.ExitError); ok {
			gitErrString = string(exitError.Stderr)
		}
		gitErrString = strings.TrimPrefix(gitErrString, "fatal: ")
		gitErrString = strings.TrimSuffix(gitErrString, "\n")
		gitErrString = strings.TrimSuffix(gitErrString, ": .git")
		g.Error = errors.New(gitErrString)
		return gitErrString
	}

	out := strings.TrimSuffix(string(output), "\n")
	return out
}
