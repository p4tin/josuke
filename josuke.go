package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
)

type Payload struct {
	Ref        string `json:"ref"`
	Action     string
	HtmlUrl    string `json:"html_url"`
	Repository struct {
		Name string `json:"full_name"`
	} `json:"repository"`
}

type Config []Repo

type Repo struct {
	Name     string   `json:"repo"`
	Branches []Branch `json:"branches"`
	Info     Info
}

type Info struct {
	BaseDir string `json:"base_dir"`
	ProjDir string `json:"proj_dir"`
	HtmlUrl string
}

type Branch struct {
	Name    string   `json:"branch"`
	Actions []Action `json:"actions"`
}

type Action struct {
	Action   string   `json:"action"`
	Commands Commands `json:"commands"`
}

type Commands [][]string

var config Config
var staticRefPrefix = "refs/heads/"

func executeCommand(c []string) error {
	if len(c) == 0 {
		return fmt.Errorf("Empy command slice")
	}
	name := c[0]
	var args []string
	if len(c) > 1 {
		args = c[1:len(c)]
	}
	cmd := exec.Command(name, args...)
	cmd.Run()
	return nil
}

func (a *Action) deploy(i *Info) {
	os.Chdir(i.BaseDir)
	if _, err := os.Stat(i.ProjDir); os.IsNotExist(err) {
		executeCommand([]string{"git", "clone", i.HtmlUrl})
	}
	os.Chdir(i.ProjDir)
	for _, command := range a.Commands {
		executeCommand(command)
	}
}

func (a Action) matches(trial string) bool {
	return a.Action == trial
}

func (b Branch) matches(trial string) bool {
	return b.Name == trial
}

func (r Repo) matches(trial string) bool {
	return r.Name == fmt.Sprintf("%s%s", staticRefPrefix, trial)
}

func (p *Payload) getAction(b *Branch) *Action {
	for _, action := range b.Actions {
		if action.matches(p.Action) {
			return &action
		}
	}
	return nil
}

func (p *Payload) getBranch(r *Repo) *Branch {
	for _, branch := range r.Branches {
		if branch.matches(p.Ref) {
			return &branch
		}
	}
	return nil
}

func (p *Payload) getRepo() *Repo {
	for _, repo := range config {
		if repo.matches(p.Repository.Name) {
			return &repo
		}
	}
	return nil
}

func (p *Payload) getDeployAction() (*Action, *Info) {
	repo := p.getRepo()
	if repo == nil {
		fmt.Println("Could not match any repo in config file. We'll just do nothing.")
		return nil, nil
	}
	branch := p.getBranch(repo)
	if repo == nil {
		fmt.Println("Could not find any matching branch. We'll just do nothing.")
		return nil, nil
	}
	// ref = fmt.Sprintf("%s%s", staticRefPrefix, )
	action := p.getAction(branch)
	if action == nil {
		fmt.Println("Could not find any matchin action. We'll just do nothing.")
		return nil, nil
	}
	repo.Info.HtmlUrl = p.HtmlUrl
	return action, &repo.Info
}

func fetchPayload(r io.Reader) *Payload {
	payload := new(Payload)
	err := json.NewDecoder(r).Decode(payload)
	if err != nil {
		panic(err)
	}
	return payload
}

func request(rw http.ResponseWriter, req *http.Request) {
	var action string
	payload := fetchPayload(req.Body)

	defer req.Body.Close()

	if action = req.Header.Get("x-github-event"); action == "" {
		return
	}

	payload.Action = action

	data, info := payload.getDeployAction()
	if data == nil {
		return
	}

	data.deploy(info)
}

func main() {
	configFileName := flag.String("c", "config.json", "Path to config file")
	port := flag.Int("p", 8082, "Port server will listen to")
	file, err := ioutil.ReadFile(*configFileName)
	if err != nil {
		log.Fatalf("Could not read config file: %v", err)
	}
	if err := json.Unmarshal(file, &config); err != nil {
		log.Fatalf("Could not parse json from config file")
	}
	http.HandleFunc("/deploy", request)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", *port), nil))
}