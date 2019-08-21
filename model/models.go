package model

import "time"

// Photo model
type Photo struct {
	ID       int       `db: id`
	Filename string    `db: filename`
	Filepath string    `db: filepath`
	Note     string    `db: "note"`
	AddedOn  time.Time `db:"added_on"`
	Public   bool      `db: "public"`
}

// Album model
type Album struct {
	ID      int       `db: id`
	Name    string    `db: name`
	Note    string    `db: "note"`
	AddedOn time.Time `db:"added_on"`
	Public  bool      `db: "public"`
}
