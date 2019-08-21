// or see here
// https://hackernoon.com/how-to-work-with-databases-in-golang-33b002aa8c47

package db

import (
	"database/sql"
	"fmt"

	"../model"
	_ "github.com/mattn/go-sqlite3"
)

// InitDB(<filepath>)
func InitDB(filepath string) *sql.DB {
	db, err := sql.Open("sqlite3", filepath)
	if err != nil {
		panic(err)
	}
	if db == nil {
		panic("db nil")
	}
	return db
}

// StorePhotos - add photos to db
func StorePhotos(db *sql.DB, items []model.Photo) {
	sqlAddPhoto := `
	INSERT INTO photo(
		filename,
		filepath,
		note
	) values(?, ?, ?)
	`

	stmt, err := db.Prepare(sqlAddPhoto)
	if err != nil {
		panic(err)
	}
	defer stmt.Close()

	for _, item := range items {
		_, err2 := stmt.Exec(item.Filename, item.Filepath, item.Note)
		if err2 != nil {
			panic(err2)
		}
	}
}

// RetrievePhotos - add photos to db
func RetrievePhotos(db *sql.DB) []model.Photo {
	sqlRetrievePhotos := fmt.Sprintf(`
	SELECT id, filename, filepath, note, added_on, public
	FROM photo
	ORDER BY added_on DESC
	LIMIT 20`)

	rows, err := db.Query(sqlRetrievePhotos)
	if err != nil {
		panic(err)
	}
	defer rows.Close()

	var result []model.Photo
	for rows.Next() {
		p := model.Photo{}
		err := rows.Scan(&p.ID, &p.Filename, &p.Filepath, &p.Note, &p.AddedOn, &p.Public)
		if err != nil {
			panic(err)
		}
		result = append(result, p)
	}
	return result
}
