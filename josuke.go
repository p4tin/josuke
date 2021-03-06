package josuke

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
)

type Josuke struct {
	GithubHook    string   `json:"github_hook"`
	BitbucketHook string   `json:"bitbucket_hook"`
	Deployment    *[]*Repo `json:"deployment"`
	Port          int      `json:"port"`
}

func New(configFilePath string) (*Josuke, error) {
	file, err := ioutil.ReadFile(configFilePath)

	if err != nil {
		return nil, fmt.Errorf("Could not read config file: %v", err)
	}

	j := &Josuke{}

	if err := json.Unmarshal(file, j); err != nil {
		return nil, errors.New("Could not parse json from config file")
	}

	return j, nil
}

var keyholders = map[string]func(*Info) string{
	"%base_dir%": func(i *Info) string {
		return i.BaseDir
	},
	"%proj_dir%": func(i *Info) string {
		return i.ProjDir
	},
	"%html_url%": func(i *Info) string {
		return i.HtmlUrl
	},
}

// Repository represents the paylaod repository informations
type Repository struct {
	Name    string `json:"full_name"`
	HtmlUrl string `json:"html_url"`
}

// Repo is built from github's json payload, mirroring dir data from config, branches & repo name
type Repo struct {
	Name     string   `json:"repo"`
	Branches []Branch `json:"branches"`
	BaseDir  string   `json:"base_dir"`
	ProjDir  string   `json:"proj_dir"`
}

// Matches repo names from payload and config
func (r Repo) matches(trial string) bool {
	return r.Name == trial
}

// Info contains various data about directory to deploy to and git's repo url
type Info struct {
	BaseDir string
	ProjDir string
	HtmlUrl string
}

// Branch mirrors config's branch section, containing branch Name & Actions linked to it
type Branch struct {
	Name    string   `json:"branch"`
	Actions []Action `json:"actions"`
}

// Matches a branch name using payload & concatenation of static "refs/heads/" + config's branch name
func (b Branch) matches(trial string) bool {
	return fmt.Sprintf("%s%s", staticRefPrefix, b.Name) == trial
}

// Action contains set of commands from config matching the type of action sent from github (if action is "push", then we do "these" commands)
type Action struct {
	Action   string     `json:"action"`
	Commands [][]string `json:"commands"`
}

// Executes the retrived set of commands from config
func (a *Action) execute(i *Info) error {
	for _, command := range a.Commands {
		if err := ExecuteCommand(command, i); err != nil {
			return err
		}
	}

	return nil
}

// Matches an action type using github's payload & config's action type
func (a Action) matches(trial string) bool {
	return a.Action == trial
}

// Config mirrors our json config file, used to boot this deployer
// var Config []Repo
var staticRefPrefix = "refs/heads/"

func fetchPayload(r io.Reader) (*Payload, error) {
	payload := &Payload{}
	err := json.NewDecoder(r).Decode(payload)
	if err != nil {
		return nil, err
	}
	return payload, nil
}

func chdir(args []string, i *Info) error {
	args = replaceKeyholders(args, i)
	if err := os.Chdir(args[0]); err != nil {
		return fmt.Errorf("%s on \"%s\" directory", err.Error(), args[0])
	}
	return nil
}

func replaceKeyholders(args []string, i *Info) []string {
	for k, arg := range args {
		if fun, ok := keyholders[arg]; ok {
			args[k] = fun(i)
		}
	}
	return args
}

// ExecuteCommand execute a command and its args coming in a form of a slice of string, using Info
func ExecuteCommand(c []string, i *Info) error {
	if len(c) == 0 {
		return fmt.Errorf("Empy command slice")
	}
	name := c[0]
	var args []string
	if len(c) > 1 {
		args = c[1:len(c)]
	}
	if name == "cd" {
		return chdir(args, i)
	}

	if name == "git" && args[0] == "clone" {
		if _, err := os.Stat(i.ProjDir); !os.IsNotExist(err) {
			return nil
		}
	}
	args = replaceKeyholders(args, i)
	cmd := exec.Command(name, args...)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("Could not execute command %s %v: %s", name, args, err)
	}
	return nil
}
