package main

// import (
// 	"context"
// 	"encoding/json"
// 	"fmt"
// 	"net/http"
// )

// func RespondWith(w http.ResponseWriter, status int, body interface{}, err string) {
// 	res := make(map[string]interface{})
// 	res["error"] = err
// 	if err == "" {
// 		res["response"] = body
// 	}
// 	bytes, _ := json.Marshal(res)
// 	w.Header().Set("Content-Type", "application/json; charset=utf-8")
// 	w.WriteHeader(status)
// 	w.Write(bytes)
// }

// func (s *MyApi) ServeHTTP(w http.ResponseWriter, r *http.Request) {
// 	switch r.URL.Path {
// 	case "/user/profile":
// 		s.handlerProfile(w, r)
// 		break
// 	case "/user/create":
// 		s.handlerCreate(w, r)
// 		break
// 	default:
// 		RespondWith(w, http.StatusNotFound, nil, "unknown method")
// 		break
// 	}
// }

// func (s *OtherApi) ServeHTTP(w http.ResponseWriter, r *http.Request) {
// 	switch r.URL.Path {
// 	case "/user/create":
// 		s.handlerCreate(w, r)
// 		break
// 	default:
// 		RespondWith(w, http.StatusNotFound, nil, "unknown method")
// 		break
// 	}
// }

// func getProfileParams(r *http.Request) ProfileParams {
// 	in := ProfileParams{}

// 	if r.Method == http.MethodPost {
// 		r.ParseForm()
// 		in.Login = r.FormValue("login")
// 	} else {
// 		in.Login = r.URL.Query().Get("login")
// 	}
// 	return in
// }

// func validateProfileParams(in *ProfileParams) error {
// 	if in.Login == "" {
// 		return fmt.Errorf("login must me not empty")
// 	}
// 	return nil
// }

// // apigen:api {"url": "/user/profile", "auth": false}
// func (s *MyApi) handlerProfile(w http.ResponseWriter, r *http.Request) {
// 	defer r.Body.Close()
// 	ctx := r.Context()
// 	if ctx == nil {
// 		ctx = context.Background()
// 	}

// 	in := getProfileParams(r)
// 	err := validateProfileParams(in)

// 	if err != nil {
// 		RespondWith(w, http.StatusBadRequest, nil, err.Error())
// 		return
// 	}

// 	res, err := s.Profile(ctx, in)
// 	if err != nil {
// 		if e, ok := err.(ApiError); ok {
// 			RespondWith(w, e.HTTPStatus, nil, e.Error())
// 		} else {
// 			RespondWith(w, http.StatusInternalServerError, nil, err.Error())
// 		}
// 		return
// 	}
// 	RespondWith(w, http.StatusOK, res, "")
// 	return
// }

// // apigen:api {"url": "/user/create", "auth": true, "method": "POST"}
// func (s *MyApi) handlerCreate(w http.ResponseWriter, r *http.Request) {

// }

// func (s *OtherApi) handlerCreate(w http.ResponseWriter, r *http.Request) {

// }
