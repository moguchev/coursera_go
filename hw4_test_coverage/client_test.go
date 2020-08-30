package main

// код писать тут

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"
)

const (
	datasetPath = "./dataset.xml"
	offset      = "offset"
	limit       = "limit"
	pageSize    = 25
)

type XMLRow struct {
	ID        int    `xml:"id"`
	Age       int    `xml:"age"`
	FirstName string `xml:"first_name"`
	LastName  string `xml:"last_name"`
	Gender    string `xml:"gender"`
	About     string `xml:"about"`
}

type XMLStructure struct {
	Version string   `xml:"version"`
	Row     []XMLRow `xml:"row"`
}

type SearchServer struct {
	repo *Repository
}

type Repository struct {
	DB string
}

var server *SearchServer

func init() {
	server = &SearchServer{
		repo: &Repository{DB: datasetPath},
	}
}

func (r *Repository) GetUsers(ctx context.Context) ([]User, error) {
	data, err := ioutil.ReadFile(r.DB)
	if err != nil {
		return nil, err
	}

	usersXML := &XMLStructure{}
	err = xml.Unmarshal(data, &usersXML)
	if err != nil {
		return nil, err
	}

	users := []User{}
	for _, user := range usersXML.Row {
		users = append(users, User{
			Id:     user.ID,
			Name:   user.FirstName + user.LastName,
			Age:    user.Age,
			About:  user.About,
			Gender: user.Gender,
		})
	}
	return users, nil
}

func (s *SearchServer) Success(w http.ResponseWriter, r *http.Request) {
	users, err := s.repo.GetUsers(r.Context())
	if err != nil {
		panic(err) // not in production
	}

	offset, _ := strconv.Atoi(r.FormValue(offset))
	limit, _ := strconv.Atoi(r.FormValue(limit))

	startRow := 0
	if offset > 0 {
		startRow = offset * pageSize
	}

	endRow := startRow + limit
	users = users[startRow:endRow]

	res, err := json.Marshal(users)
	if err != nil {
		panic(err) // not in production
	}

	w.WriteHeader(http.StatusOK)
	w.Write(res)
}

func (s *SearchServer) LimitFail(w http.ResponseWriter, r *http.Request) {
	users, err := s.repo.GetUsers(r.Context())
	if err != nil {
		panic(err) // not in production
	}

	res, err := json.Marshal(users)
	if err != nil {
		panic(err) // not in production
	}

	w.WriteHeader(http.StatusOK)
	w.Write(res)
}

func (s *SearchServer) JSONFail(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	io.WriteString(w, `"err": "bad json"}`)
}

func (s *SearchServer) TimeoutError(w http.ResponseWriter, r *http.Request) {
	time.Sleep(time.Second * 2)
	w.WriteHeader(http.StatusOK)
}

func (s *SearchServer) UnknownError(w http.ResponseWriter, r *http.Request) {}

func (s *SearchServer) Unauthorized(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusUnauthorized)
}

func (s *SearchServer) InternalServerError(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusInternalServerError)
}

func (s *SearchServer) BadRequest(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusBadRequest)
}

func (s *SearchServer) BadField(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusBadRequest)
	res, _ := json.Marshal(SearchErrorResponse{Error: "ErrorBadOrderField"})
	w.Write(res)
}

func (s *SearchServer) BadError(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusBadRequest)
	res, _ := json.Marshal(SearchErrorResponse{Error: "Unknown error"})
	w.Write(res)
}

func TestErrorResponse(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(server.Success))

	searchClient := &SearchClient{
		URL: ts.URL,
	}

	searchRequest := SearchRequest{
		Limit:  5,
		Offset: 0,
	}

	_, err := searchClient.FindUsers(searchRequest)

	if err != nil {
		t.Error("Dosn't work success request")
	}

	searchRequest.Limit = -1

	_, err = searchClient.FindUsers(searchRequest)
	if err.Error() != "limit must be > 0" {
		t.Error("limit must be > 0")
	}

	searchRequest.Limit = 1
	searchRequest.Offset = -1
	_, err = searchClient.FindUsers(searchRequest)
	if err.Error() != "offset must be > 0" {
		t.Error("offset must be > 0")
	}

	ts.Close()
}

func TestLimitFailed(t *testing.T) {
	limit := 7
	ts := httptest.NewServer(http.HandlerFunc(server.LimitFail))

	searchClient := &SearchClient{
		URL: ts.URL,
	}

	response, _ := searchClient.FindUsers(SearchRequest{Limit: limit})

	if limit == len(response.Users) {
		t.Error("Limit not true")
	}
	ts.Close()
}

func TestBadJson(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(server.JSONFail))
	searchClient := &SearchClient{
		URL: ts.URL,
	}
	_, err := searchClient.FindUsers(SearchRequest{})

	if err.Error() != `cant unpack result json: invalid character ':' after top-level value` {
		t.Error("Bad json test :(")
	}
	ts.Close()
}

func TestPerelimit(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(server.Success))
	searchClient := &SearchClient{
		URL: ts.URL,
	}

	response, _ := searchClient.FindUsers(SearchRequest{Limit: 26})

	if 25 != len(response.Users) {
		t.Error("Perelimit :(")
	}
	ts.Close()
}

func TestTimeoutError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(server.TimeoutError))
	searchClient := &SearchClient{
		URL: ts.URL,
	}

	_, err := searchClient.FindUsers(SearchRequest{})

	if err == nil {
		t.Error("Timeout chck error :(")
	}

	ts.Close()
}

func TestUnknownError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(server.UnknownError))
	searchClient := &SearchClient{
		URL: "bad_link",
	}

	_, err := searchClient.FindUsers(SearchRequest{})

	if err == nil {
		t.Error("TestUnknownError :(")
	}

	ts.Close()
}

func TestStatusUnauthorized(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(server.Unauthorized))
	searchClient := &SearchClient{URL: ts.URL}
	_, err := searchClient.FindUsers(SearchRequest{})

	if err.Error() != "Bad AccessToken" {
		t.Error("Bad AccessToken is not done :(")
	}

	ts.Close()
}

func TestStatusInternalServerError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(server.InternalServerError))
	searchClient := &SearchClient{URL: ts.URL}
	_, err := searchClient.FindUsers(SearchRequest{})

	if err.Error() != "SearchServer fatal error" {
		t.Error("SearchServer fatal error is not done :(")
	}

	ts.Close()
}

func TestBadRequest(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(server.BadRequest))
	searchClient := &SearchClient{URL: ts.URL}
	_, err := searchClient.FindUsers(SearchRequest{})

	if err.Error() != "cant unpack error json: unexpected end of JSON input" {
		t.Error("TestBadRequest is not done")
	}

	ts.Close()
}

func TestBadField(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(server.BadField))
	searchClient := &SearchClient{URL: ts.URL}
	_, err := searchClient.FindUsers(SearchRequest{})
	if err.Error() != "OrderFeld  invalid" {
		t.Error("ErrorBadOrderField is not done")
	}

	ts.Close()
}

func TestBadRequestError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(server.BadError))
	searchClient := &SearchClient{URL: ts.URL}
	_, err := searchClient.FindUsers(SearchRequest{})
	if err == nil {
		t.Error("TestBadRequestError is not done")
	}

	ts.Close()
}
