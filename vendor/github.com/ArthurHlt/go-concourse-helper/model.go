package go_concourse_helper

type Version struct {
	BuildNumber string `json:"build"`
}

type Metadata struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}
type Request struct {
	Source  interface{} `json:"source"`
	Version Version     `json:"version"`
	Params  interface{} `json:"params"`
}

type Response struct {
	Version  Version    `json:"version"`
	Metadata []Metadata `json:"metadata"`
}
