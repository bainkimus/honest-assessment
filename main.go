package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gorilla/mux"
	env "github.com/joho/godotenv"
)

const envFile = ".env"
const dataFile = "data/forms.json"

var loadEnv = env.Load

type formInput struct {
	FirstName   string `json:"first_name"`
	LastName    string `json:"last_name"`
	Email       string `json:"email"`
	PhoneNumber string `json:"phone_number"`
}

func (f formInput) validate() error {
	if f.FirstName == "" || f.LastName == "" || f.Email == "" || f.PhoneNumber == "" {
		return errors.New("invalid input")
	}
	return nil
}

func getData() (*[]formInput, error) {
	file, err := ioutil.ReadFile(dataFile)
	if err != nil {
		return nil, err
	}
	var forms []formInput
	err = json.Unmarshal(file, &forms)
	if err != nil {
		return nil, err
	}
	return &forms, nil
}

func (f formInput) save() error {

	formDatas, err := getData()
	if err != nil {
		return err
	}

	forms := append(*formDatas, f)
	toSave, err := json.Marshal(forms)
	if err != nil {
		return err
	}
	err = ioutil.WriteFile(dataFile, toSave, os.ModeAppend)
	return err
}

func handleFunc(resp http.ResponseWriter, req *http.Request) {

	switch req.Method {
	case http.MethodPost:
		log.Println("MethodPost")
		if err := req.ParseForm(); err != nil {
			resp.WriteHeader(http.StatusBadRequest)
			fmt.Fprint(resp, err.Error())
			return
		}

		f := formInput{
			FirstName:   req.FormValue("first_name"),
			LastName:    req.FormValue("last_name"),
			Email:       req.FormValue("email"),
			PhoneNumber: req.FormValue("phone_number"),
		}

		if err := f.validate(); err != nil {
			resp.WriteHeader(http.StatusBadRequest)
			fmt.Fprint(resp, err.Error())
			return
		}

		if err := f.save(); err != nil {
			resp.WriteHeader(http.StatusInternalServerError)
			fmt.Fprint(resp, err.Error())
			return
		}
		resp.WriteHeader(http.StatusOK)
		fmt.Fprint(resp, "form saved ")
	case http.MethodGet:
		log.Println("MethodGet")
		formDatas, err := getData()
		if err != nil {
			resp.WriteHeader(http.StatusInternalServerError)
			fmt.Fprint(resp, err.Error())
			return
		}
		resp.WriteHeader(http.StatusOK)
		json.NewEncoder(resp).Encode(formDatas)
	default:
		log.Println("error no 404")
		resp.WriteHeader(http.StatusNotFound)
		fmt.Fprint(resp, "not found")
	}
}

func form(resp http.ResponseWriter, req *http.Request) {
	tmpl := template.Must(template.ParseFiles("form.html"))

	tmpl.Execute(resp, nil)
}

func run() (s *http.Server) {
	err := loadEnv(envFile)
	if err != nil {
		log.Fatal(err)
	}
	port, exist := os.LookupEnv("PORT")
	if !exist {
		log.Fatal("no port specified")
	}
	port = fmt.Sprintf(":%s", port)
	router := mux.NewRouter()
	router.HandleFunc("/", handleFunc)
	router.HandleFunc("/addData", form)

	s = &http.Server{
		Handler:        router,
		Addr:           port,
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}

	go func() {

		log.Printf("listening on port ( %s )\n", port)
		log.Printf("Add data path ( %s )\n", "/addData")

		if err := s.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %s\n", err)
		}
	}()

	return
}

func main() {
	s := run()
	quit := make(chan os.Signal)
	// kill (no param) default send syscall.SIGTERM
	// kill -2 is syscall.SIGINT
	// kill -9 is syscall.SIGKILL but can't be catch, so don't need add it
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutting down server...")

	// The context is used to inform the server it has 5 seconds to finish
	// the request it is currently handling
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer func() {
		cancel()
	}()
	if err := s.Shutdown(ctx); err != nil {
		log.Fatal("Server forced to shutdown")
	}
	log.Println("Server exiting")
}
