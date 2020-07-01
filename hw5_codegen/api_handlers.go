package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
)

func RespondWith(w http.ResponseWriter, status int, body interface{}, err string) {
	res := make(map[string]interface{})
	res["error"] = err
	if err == "" {
		res["response"] = body
	}
	bytes, _ := json.Marshal(res)
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	w.Write(bytes)
}

func (s *MyApi) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/user/profile":
		s.handlerProfile(w, r)
		break
	case "/user/create":
		s.handlerCreate(w, r)
		break
	default:
		RespondWith(w, http.StatusNotFound, nil, "unknown method")
		break
	}
}

func (s *OtherApi) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/user/create":
		s.handlerCreate(w, r)
		break
	default:
		RespondWith(w, http.StatusNotFound, nil, "unknown method")
		break
	}
}

func (s *MyApi) handlerProfile(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	ctx := r.Context()
	if ctx == nil {
		ctx = context.Background()
	}

	in, e := getProfileParams(r)
	if e != nil {
		RespondWith(w, http.StatusBadRequest, nil, e.Error())
		return
	}
	err := validateProfileParams(&in)

	if err != nil {
		RespondWith(w, http.StatusBadRequest, nil, err.Error())
		return
	}

	res, err := s.Profile(ctx, in)
	if err != nil {
		if e, ok := err.(ApiError); ok {
			RespondWith(w, e.HTTPStatus, nil, e.Error())
		} else {
			RespondWith(w, http.StatusInternalServerError, nil, err.Error())
		}
		return
	}
	RespondWith(w, http.StatusOK, res, "")
	return
}

func (s *MyApi) handlerCreate(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	ctx := r.Context()
	if ctx == nil {
		ctx = context.Background()
	}

	if r.Method != http.MethodPost {
		RespondWith(w, http.StatusNotAcceptable, nil, "bad method")
		return
	}

	auth := r.Header.Get("X-Auth")
	if auth != "100500" {
		RespondWith(w, http.StatusForbidden, nil, "unauthorized")
		return
	}

	in, e := getCreateParams(r)
	if e != nil {
		RespondWith(w, http.StatusBadRequest, nil, e.Error())
		return
	}
	err := validateCreateParams(&in)

	if err != nil {
		RespondWith(w, http.StatusBadRequest, nil, err.Error())
		return
	}

	res, err := s.Create(ctx, in)
	if err != nil {
		if e, ok := err.(ApiError); ok {
			RespondWith(w, e.HTTPStatus, nil, e.Error())
		} else {
			RespondWith(w, http.StatusInternalServerError, nil, err.Error())
		}
		return
	}
	RespondWith(w, http.StatusOK, res, "")
	return
}

func (s *OtherApi) handlerCreate(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	ctx := r.Context()
	if ctx == nil {
		ctx = context.Background()
	}

	if r.Method != http.MethodPost {
		RespondWith(w, http.StatusNotAcceptable, nil, "bad method")
		return
	}

	auth := r.Header.Get("X-Auth")
	if auth != "100500" {
		RespondWith(w, http.StatusForbidden, nil, "unauthorized")
		return
	}

	in, e := getOtherCreateParams(r)
	if e != nil {
		RespondWith(w, http.StatusBadRequest, nil, e.Error())
		return
	}
	err := validateOtherCreateParams(&in)

	if err != nil {
		RespondWith(w, http.StatusBadRequest, nil, err.Error())
		return
	}

	res, err := s.Create(ctx, in)
	if err != nil {
		if e, ok := err.(ApiError); ok {
			RespondWith(w, e.HTTPStatus, nil, e.Error())
		} else {
			RespondWith(w, http.StatusInternalServerError, nil, err.Error())
		}
		return
	}
	RespondWith(w, http.StatusOK, res, "")
	return
}

func getProfileParams(r *http.Request) (ProfileParams, error) {
	in := ProfileParams{}
	var err error

	if r.Method == http.MethodPost {
		r.ParseForm()
		in.Login = r.FormValue("login")
	} else {
		in.Login = r.URL.Query().Get("login")
	}
	return in, err
}

func validateProfileParams(in *ProfileParams) error {

	if in.Login == "" {
		return fmt.Errorf("login must me not empty")
	}

	return nil
}

func getCreateParams(r *http.Request) (CreateParams, error) {
	in := CreateParams{}
	var err error

	if r.Method == http.MethodPost {
		r.ParseForm()
		in.Login = r.FormValue("login")
		in.Name = r.FormValue("full_name")
		in.Status = r.FormValue("status")
		in.Age, err = strconv.Atoi(r.FormValue("age"))
		if err != nil {
			return in, fmt.Errorf("age must be int")
		}
	} else {
		in.Login = r.URL.Query().Get("login")
		in.Name = r.URL.Query().Get("full_name")
		in.Status = r.URL.Query().Get("status")
		in.Age, err = strconv.Atoi(r.URL.Query().Get("age"))
		if err != nil {
			return in, fmt.Errorf("age must be int")
		}
	}
	return in, err
}

func validateCreateParams(in *CreateParams) error {

	if in.Login == "" {
		return fmt.Errorf("login must me not empty")
	}

	if len(in.Login) < 10 {
		return fmt.Errorf("login len must be >= 10")
	}

	if in.Status == "" {
		in.Status = "user"
	}
	ok := false
	enum := []string{"user", "moderator", "admin"}
	for _, e := range enum {
		if in.Status == e {
			ok = true
			break
		}
	}
	if !ok {
		return fmt.Errorf("status must be one of [%s]", `user, moderator, admin`)
	}

	if in.Age > 128 {
		return fmt.Errorf("age must be <= 128")
	}
	if in.Age < 0 {
		return fmt.Errorf("age must be >= 0")
	}
	return nil
}

func getOtherCreateParams(r *http.Request) (OtherCreateParams, error) {
	in := OtherCreateParams{}
	var err error

	if r.Method == http.MethodPost {
		r.ParseForm()
		in.Username = r.FormValue("username")
		in.Name = r.FormValue("account_name")
		in.Class = r.FormValue("class")
		in.Level, err = strconv.Atoi(r.FormValue("level"))
		if err != nil {
			return in, fmt.Errorf("level must be int")
		}
	} else {
		in.Username = r.URL.Query().Get("username")
		in.Name = r.URL.Query().Get("account_name")
		in.Class = r.URL.Query().Get("class")
		in.Level, err = strconv.Atoi(r.URL.Query().Get("level"))
		if err != nil {
			return in, fmt.Errorf("level must be int")
		}
	}
	return in, err
}

func validateOtherCreateParams(in *OtherCreateParams) error {

	if in.Username == "" {
		return fmt.Errorf("username must me not empty")
	}

	if len(in.Username) < 3 {
		return fmt.Errorf("username len must be >= 3")
	}

	if in.Class == "" {
		in.Class = "warrior"
	}
	ok := false
	enum := []string{"warrior", "sorcerer", "rouge"}
	for _, e := range enum {
		if in.Class == e {
			ok = true
			break
		}
	}
	if !ok {
		return fmt.Errorf("class must be one of [%s]", `warrior, sorcerer, rouge`)
	}

	if in.Level > 50 {
		return fmt.Errorf("level must be <= 50")
	}
	if in.Level < 1 {
		return fmt.Errorf("level must be >= 1")
	}
	return nil
}
