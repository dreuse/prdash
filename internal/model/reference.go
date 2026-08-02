package model

type Issue struct {
	Repo   string
	Number int
	Title  string
	URL    string
}

type User struct {
	Login string
	Name  string
}
