package main

import (
	"crypto/rand"
	"errors"
	"fmt"
	"html/template"
	"image/jpeg"
	"io/ioutil"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"path"
	"reflect"
	"runtime"
	"strings"

	"./db"
	"./model"
	"github.com/nfnt/resize"
)

const dbpath = "db.db"

var uploadDir string
var templates *template.Template

var dbUsers = map[string]model.User{
	"cornel": {ID: 1, UserName: "admin", Password: "admin"},
}                                    // username, user
var dbSessions = map[string]string{} // session ID, username

func xlog(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		name := runtime.FuncForPC(reflect.ValueOf(h).Pointer()).Name()
		log.Printf(" - uri:%s\thandler:%s\n", r.RequestURI, name)
		h(w, r)
	}
}

func index(w http.ResponseWriter, r *http.Request) {
	u := getUser(w, r)
	log.Println("u:", u)

	pageData := struct {
		User         model.User
		ShowLoginBox bool
	}{}
	if u.ID != 0 {
		pageData.User = u
	} else {
		pageData.ShowLoginBox = true
	}
	t := templates.Lookup("index.gohtml")
	//log.Println(t.Name())
	log.Println("data =", pageData)
	t.ExecuteTemplate(w, "index.gohtml", pageData)
}

func login(w http.ResponseWriter, r *http.Request) {
	//u := getUser(w, r)
	//log.Println(r.Method)
	if r.Method == "POST" {
		err := r.ParseForm()
		if err != nil {
			log.Println(err)
		}
		un := strings.TrimSpace(r.FormValue("username"))
		p := strings.TrimSpace(r.FormValue("password"))
		log.Println(un, " : ", p)
		// is there such username?
		u, ok := dbUsers[un]
		log.Println(ok, u)
		if !ok {
			http.Error(w, "Username and/or password do not match", http.StatusForbidden)
			return
		}
		if p != u.Password {
			http.Error(w, "Username and/or password do not match", http.StatusForbidden)
			return
		}

		// create session
		c := &http.Cookie{
			Name:     "session",
			Value:    _uuid(),
			HttpOnly: true,
		}
		http.SetCookie(w, c)
		dbSessions[c.Value] = un
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func logout(w http.ResponseWriter, r *http.Request) {
	if !alreadyLoggedIn(r) {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	c, _ := r.Cookie("session")
	// delete the session
	delete(dbSessions, c.Value)
	// remove the cookie
	c = &http.Cookie{
		Name:     "session",
		Value:    "",
		MaxAge:   -1,
		HttpOnly: true,
	}
	http.SetCookie(w, c)

	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

func listPhotos(w http.ResponseWriter, r *http.Request) {
	u := getUser(w, r)
	dbh := db.InitDB(dbpath)
	defer dbh.Close()

	allPhotos := db.RetrievePhotos(dbh)
	type thumbPhoto struct {
		model.Photo
		Thumb string
	}

	thumbs := make([]thumbPhoto, len(allPhotos))
	for i, p := range allPhotos {
		dirname, fname := path.Split(p.Filepath)
		thumb := dirname + "t_" + fname
		thumbs[i] = thumbPhoto{
			p,
			thumb,
		}
	}
	pageData := struct {
		User   model.User
		Thumbs []thumbPhoto
	}{
		Thumbs: thumbs,
		User:   u,
	}
	t := templates.Lookup("list_photos.gohtml")
	log.Println(t.Name())
	t.ExecuteTemplate(w, "list_photos.gohtml", pageData)
}

func addPhotos(w http.ResponseWriter, r *http.Request) {

	u := getUser(w, r)
	if !alreadyLoggedIn(r) {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	log.Println("size: ", r.Header.Get("Content-length"))
	r.Body = http.MaxBytesReader(w, r.Body, 10<<21)
	//log.Panicln("size: ", len(r.Body))
	msg := ""
	if r.Method == "POST" {
		// Parse our multipart form, 10 << 21 specifies a maximum
		// upload of 20 MB files.
		if err := r.ParseMultipartForm(10 << 10); nil != err {
			log.Println("eroare >> ", err)
			//http.Error(w, err.Error(), http.StatusInternalServerError)
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(fmt.Sprintf("500 - %s", err)))
			return
		}

		processedPhotos := []model.Photo{}

		//fmt.Fprintln(w, "---------------------\n")
		files := r.MultipartForm.File["photos"]
		for i := range files { //Iterate over multiple uploaded files

			//log.Printf("type of files[i] = %T", files[i])
			//ext := path.Ext(files[i].Filename)
			//log.Println("file ext: ", ext)
			//log.Println("+ adding file: ", files[i].Filename)
			userNote := strings.TrimSpace(r.FormValue("note"))
			photo, err := processUploadedFile(files[i], userNote)
			log.Println(photo)
			log.Println(err)
			if err != nil {
				msg += fmt.Sprintf("\nError: %s", err.Error())
				continue
			}

			photo.Note = strings.TrimSpace(r.FormValue("note"))
			processedPhotos = append(processedPhotos, photo)
		}
		if len(processedPhotos) > 0 {
			dbh := db.InitDB(dbpath)
			defer dbh.Close()

			db.StorePhotos(dbh, processedPhotos)
			msg += fmt.Sprintf("\nadded %d photos", len(processedPhotos))
		}
	}

	log.Println("method: ", r.Method)
	t := templates.Lookup("add_photo.gohtml")
	log.Println(t.Name())
	pageData := struct {
		User    model.User
		Message string
	}{u, msg}
	t.ExecuteTemplate(w, "add_photo.gohtml", pageData)
}

func _uuid() string {
	b := make([]byte, 16)
	_, err := rand.Read(b)
	if err != nil {
		log.Fatal(err)
	}
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}

func processUploadedFile(fh *multipart.FileHeader, userNote string) (model.Photo, error) {
	log.Println(".... ", fh.Filename)
	msg := ""
	file, err := fh.Open()
	defer file.Close()
	if err != nil {
		log.Println("error reading file ", err)
		msg = fmt.Sprintf("Error reading file %s", fh.Filename)
		return model.Photo{}, errors.New(msg)
	}
	buff := make([]byte, 512) // docs tell that it take only first 512 bytes into consideration
	if _, err = file.Read(buff); err != nil {
		fmt.Println(err) // do something with that error
		return model.Photo{}, errors.New("zzz")
	}

	mimeType := http.DetectContentType(buff)
	if stringInSlice(mimeType, []string{"image/jpeg", "image/jpg"}) {
		log.Println("Mime Type ok")
	} else {
		log.Println("Mime Type NOT ok: ", mimeType)
		return model.Photo{}, fmt.Errorf("Uploaded file is not JPEG (%s)", mimeType)
	}

	uuidFilename := _uuid()
	fullFilePath := uploadDir + string(os.PathSeparator) + uuidFilename + ".jpg"
	thumbFilePath := uploadDir + string(os.PathSeparator) + "t_" + uuidFilename + ".jpg"
	dst, err := os.Create(fullFilePath)
	defer dst.Close()
	if err != nil {
		log.Println("error creating destination ", err)
		return model.Photo{}, err
	}
	/* -- don't save the original
	//copy the uploaded file to the destination file
	if _, err := io.Copy(dst, file); err != nil {
		fmt.Println("error copying file", err)
		return model.Photo{}, err
	}
	*/

	file.Seek(0, 0)
	// decode jpeg into image.Image
	img, err := jpeg.Decode(file)
	if err != nil {
		log.Println("error decoding image: ", err)
		//msg += "Error.. Probably not a jpeg image..\n"
		return model.Photo{}, err
	}

	m := resize.Resize(1600, 0, img, resize.Bicubic)
	err = jpeg.Encode(dst, m, nil)
	if err != nil {
		log.Println(err)
		return model.Photo{}, err
	}

	// resize to width 200 using Bicubic resampling and preserve aspect ratio
	m = resize.Resize(200, 0, img, resize.Bicubic)
	//log.Println(m)
	thumb, err := os.Create(thumbFilePath)
	if err != nil {
		log.Fatal(err)
	}
	defer thumb.Close()

	// write new image to file
	err = jpeg.Encode(thumb, m, nil)
	if err != nil {
		log.Println(err)
		return model.Photo{}, err
	}

	photo := model.Photo{
		Note:     userNote,
		Filename: fh.Filename,
		Filepath: fullFilePath,
	}

	return photo, nil
}

func faviconHandler(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "static/favicon.ico")
}

func stringInSlice(a string, list []string) bool {
	for _, b := range list {
		if b == a {
			return true
		}
	}
	return false
}

func getUser(w http.ResponseWriter, req *http.Request) model.User {
	// get cookie
	c, err := req.Cookie("session")
	if err != nil {
		//sID, _ := uuid.NewV4()
		c = &http.Cookie{
			Name:  "session",
			Value: _uuid(),
			//Value: sID.String(),
		}

	}
	//Next line may not be required, commenting it
	// http.SetCookie(w, c)

	// if the user exists already, get user
	var u model.User
	if un, ok := dbSessions[c.Value]; ok {
		u = dbUsers[un]
	}
	return u
}

func alreadyLoggedIn(req *http.Request) bool {
	c, err := req.Cookie("session")
	if err != nil {
		return false
	}
	un := dbSessions[c.Value]
	_, ok := dbUsers[un]
	return ok
}

func init() {

	uploadDir = os.Getenv("UPLOAD_DIR")
	if uploadDir == "" {
		uploadDir = "upload"
	}

	tmplDir := os.Getenv("TMPL_DIR")
	if tmplDir == "" {
		tmplDir = "tmpl"
	}

	log.Println("tmpl dir: ", tmplDir)
	log.Println("upload dir: ", uploadDir)

	var allFiles []string
	files, err := ioutil.ReadDir(tmplDir)
	if err != nil {
		fmt.Println(err)
	}
	for _, file := range files {
		log.Printf(" + found file %s", file.Name())
		filename := file.Name()
		if strings.HasSuffix(filename, ".gohtml") {
			allFiles = append(allFiles, tmplDir+"/"+filename)
		}
	}
	templates = template.Must(template.ParseFiles(allFiles...))
}

func main() {

	host := os.Getenv("HOST")
	if host == "" {
		host = "127.0.0.1"
	}
	port := os.Getenv("PORT")
	if port == "" {
		port = "8000"
	}
	log.Output(1, "starting server on "+host+":"+port)
	server := http.Server{
		Addr: host + ":" + port,
	}
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("./static"))))
	http.HandleFunc("/photos/add", xlog(addPhotos))
	http.HandleFunc("/photos", xlog(listPhotos))
	http.HandleFunc("/favicon.ico", faviconHandler)
	http.HandleFunc("/login", xlog(login))
	http.HandleFunc("/logout", xlog(logout))
	http.HandleFunc("/", xlog(index))

	//server.ListenAndServe()

	if err := server.ListenAndServe(); err != nil {
		log.Println("Error: ", err)
		os.Exit(1)
	}
}
