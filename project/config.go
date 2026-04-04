package project

// Config holds the project configuration loaded from .ks-project.json.
type Config struct {
	Name    string   `json:"name"`
	BaseURL string   `json:"baseURL"`
	URLs    []string `json:"urls"`
}

// Load reads .ks-project.json from the current working directory and returns the parsed Config.
// Returns an error wrapping a hint to run 'ks init' if the file is missing.
// Returns an error if the urls field is empty or the JSON is malformed.
func Load() (*Config, error) {
	panic("not implemented")
}
