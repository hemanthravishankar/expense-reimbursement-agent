package models

type Employee struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Grade      string `json:"grade"`
	Department string `json:"department"`
	Location   string `json:"location"`
}
