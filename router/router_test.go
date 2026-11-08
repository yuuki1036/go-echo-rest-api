package router

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"go-echo-rest-api/controller"
	"go-echo-rest-api/model"

	"github.com/golang-jwt/jwt/v5"
	"github.com/labstack/echo/v4"
)

// テスト用のダミーの署名鍵。本番の値とは無関係。
const testSecret = "test-secret-for-router-tests"

type fakeUserUsecase struct {
	signUpRes model.UserResponse
	loginRes  string
	err       error
	gotUser   model.User
}

func (f *fakeUserUsecase) SignUp(user model.User) (model.UserResponse, error) {
	f.gotUser = user
	return f.signUpRes, f.err
}

func (f *fakeUserUsecase) Login(user model.User) (string, error) {
	f.gotUser = user
	return f.loginRes, f.err
}

type fakeTaskUsecase struct {
	tasks     []model.TaskResponse
	task      model.TaskResponse
	err       error
	gotUserId uint
	gotTaskId uint
	gotTask   model.Task
}

func (f *fakeTaskUsecase) GetAllTasks(userId uint) ([]model.TaskResponse, error) {
	f.gotUserId = userId
	return f.tasks, f.err
}

func (f *fakeTaskUsecase) GetTaskById(userId uint, taskId uint) (model.TaskResponse, error) {
	f.gotUserId, f.gotTaskId = userId, taskId
	return f.task, f.err
}

func (f *fakeTaskUsecase) CreateTask(task model.Task) (model.TaskResponse, error) {
	f.gotTask = task
	f.gotUserId = task.UserId
	return f.task, f.err
}

func (f *fakeTaskUsecase) UpdateTask(task model.Task, userId uint, taskId uint) (model.TaskResponse, error) {
	f.gotTask = task
	f.gotUserId, f.gotTaskId = userId, taskId
	return f.task, f.err
}

func (f *fakeTaskUsecase) DeleteTask(userId uint, taskId uint) error {
	f.gotUserId, f.gotTaskId = userId, taskId
	return f.err
}

func newTestServer(t *testing.T, uu *fakeUserUsecase, tu *fakeTaskUsecase) *echo.Echo {
	t.Helper()
	t.Setenv("SECRET", testSecret)
	t.Setenv("API_DOMAIN", "localhost")
	return NewRouter(controller.NewUserController(uu), controller.NewTaskController(tu))
}

func signToken(t *testing.T, secret string, userId uint) string {
	t.Helper()
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id": userId,
		"exp":     time.Now().Add(time.Hour).Unix(),
	})
	s, err := token.SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}
	return s
}

func doRequest(e *echo.Echo, method, path, body, token string) *httptest.ResponseRecorder {
	var req *http.Request
	if body != "" {
		req = httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	} else {
		req = httptest.NewRequest(method, path, nil)
	}
	if token != "" {
		req.AddCookie(&http.Cookie{Name: "token", Value: token})
	}
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

func findCookie(rec *httptest.ResponseRecorder, name string) *http.Cookie {
	for _, c := range rec.Result().Cookies() {
		if c.Name == name {
			return c
		}
	}
	return nil
}

func TestSignUp(t *testing.T) {
	t.Run("成功すると 201 とユーザーを返す", func(t *testing.T) {
		uu := &fakeUserUsecase{signUpRes: model.UserResponse{ID: 1, Email: "a@example.com"}}
		e := newTestServer(t, uu, &fakeTaskUsecase{})

		rec := doRequest(e, http.MethodPost, "/signup", `{"email":"a@example.com","password":"pw"}`, "")

		if rec.Code != http.StatusCreated {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusCreated)
		}
		var got model.UserResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if got != uu.signUpRes {
			t.Errorf("body = %+v, want %+v", got, uu.signUpRes)
		}
		if uu.gotUser.Email != "a@example.com" || uu.gotUser.Password != "pw" {
			t.Errorf("usecase に渡した user = %+v", uu.gotUser)
		}
	})

	t.Run("JSON が壊れていると 400", func(t *testing.T) {
		e := newTestServer(t, &fakeUserUsecase{}, &fakeTaskUsecase{})
		rec := doRequest(e, http.MethodPost, "/signup", `{"email":`, "")
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
		}
	})

	t.Run("usecase がエラーを返すと 500", func(t *testing.T) {
		e := newTestServer(t, &fakeUserUsecase{err: errors.New("duplicate")}, &fakeTaskUsecase{})
		rec := doRequest(e, http.MethodPost, "/signup", `{"email":"a@example.com","password":"pw"}`, "")
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
		}
	})
}

func TestLogin(t *testing.T) {
	t.Run("成功すると 200 と token の cookie を返す", func(t *testing.T) {
		uu := &fakeUserUsecase{loginRes: "dummy-token"}
		e := newTestServer(t, uu, &fakeTaskUsecase{})

		rec := doRequest(e, http.MethodPost, "/login", `{"email":"a@example.com","password":"pw"}`, "")

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
		}
		c := findCookie(rec, "token")
		if c == nil {
			t.Fatal("token の cookie が無い")
		}
		if c.Value != "dummy-token" {
			t.Errorf("cookie value = %q, want %q", c.Value, "dummy-token")
		}
		if !c.HttpOnly {
			t.Error("cookie が HttpOnly でない")
		}
		if c.Path != "/" {
			t.Errorf("cookie path = %q, want %q", c.Path, "/")
		}
	})

	t.Run("usecase がエラーを返すと 500 で cookie を返さない", func(t *testing.T) {
		e := newTestServer(t, &fakeUserUsecase{err: errors.New("bad password")}, &fakeTaskUsecase{})
		rec := doRequest(e, http.MethodPost, "/login", `{"email":"a@example.com","password":"x"}`, "")
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
		}
		if findCookie(rec, "token") != nil {
			t.Error("失敗時に token の cookie を返している")
		}
	})
}

func TestLogOut(t *testing.T) {
	e := newTestServer(t, &fakeUserUsecase{}, &fakeTaskUsecase{})
	rec := doRequest(e, http.MethodPost, "/logout", "", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	c := findCookie(rec, "token")
	if c == nil {
		t.Fatal("token の cookie が無い")
	}
	if c.Value != "" {
		t.Errorf("cookie value = %q, want empty", c.Value)
	}
}

func TestTasksRequireJWT(t *testing.T) {
	tests := []struct {
		name  string
		token func(t *testing.T) string
	}{
		{"cookie が無い", func(t *testing.T) string { return "" }},
		{"別の鍵で署名した token", func(t *testing.T) string { return signToken(t, "other-secret", 1) }},
		{"token の形をしていない", func(t *testing.T) string { return "not-a-jwt" }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tu := &fakeTaskUsecase{}
			e := newTestServer(t, &fakeUserUsecase{}, tu)
			rec := doRequest(e, http.MethodGet, "/tasks", "", tt.token(t))
			if rec.Code != http.StatusUnauthorized {
				t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
			}
			if tu.gotUserId != 0 {
				t.Error("認証に失敗したのに usecase が呼ばれた")
			}
		})
	}
}

func TestTasks(t *testing.T) {
	now := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	task := model.TaskResponse{ID: 7, Title: "write tests", CreatedAt: now, UpdatedAt: now}

	t.Run("GET /tasks はトークンの user_id で一覧を返す", func(t *testing.T) {
		tu := &fakeTaskUsecase{tasks: []model.TaskResponse{task}}
		e := newTestServer(t, &fakeUserUsecase{}, tu)

		rec := doRequest(e, http.MethodGet, "/tasks", "", signToken(t, testSecret, 3))

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
		}
		if tu.gotUserId != 3 {
			t.Errorf("userId = %d, want 3", tu.gotUserId)
		}
		var got []model.TaskResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if len(got) != 1 || got[0].ID != 7 || got[0].Title != "write tests" {
			t.Errorf("body = %+v", got)
		}
	})

	t.Run("GET /tasks/:taskId は user_id と taskId を渡す", func(t *testing.T) {
		tu := &fakeTaskUsecase{task: task}
		e := newTestServer(t, &fakeUserUsecase{}, tu)

		rec := doRequest(e, http.MethodGet, "/tasks/7", "", signToken(t, testSecret, 3))

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
		}
		if tu.gotUserId != 3 || tu.gotTaskId != 7 {
			t.Errorf("userId, taskId = %d, %d, want 3, 7", tu.gotUserId, tu.gotTaskId)
		}
	})

	t.Run("POST /tasks は user_id をトークンから付ける", func(t *testing.T) {
		tu := &fakeTaskUsecase{task: task}
		e := newTestServer(t, &fakeUserUsecase{}, tu)

		// body の user_id はトークンの値で上書きされる
		rec := doRequest(e, http.MethodPost, "/tasks", `{"title":"write tests","user_id":99}`, signToken(t, testSecret, 3))

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
		}
		if tu.gotTask.Title != "write tests" {
			t.Errorf("title = %q", tu.gotTask.Title)
		}
		if tu.gotTask.UserId != 3 {
			t.Errorf("UserId = %d, want 3", tu.gotTask.UserId)
		}
	})

	t.Run("POST /tasks は JSON が壊れていると 400", func(t *testing.T) {
		e := newTestServer(t, &fakeUserUsecase{}, &fakeTaskUsecase{})
		rec := doRequest(e, http.MethodPost, "/tasks", `{"title":`, signToken(t, testSecret, 3))
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
		}
	})

	t.Run("PUT /tasks/:taskId は title と ID を渡す", func(t *testing.T) {
		tu := &fakeTaskUsecase{task: task}
		e := newTestServer(t, &fakeUserUsecase{}, tu)

		rec := doRequest(e, http.MethodPut, "/tasks/7", `{"title":"updated"}`, signToken(t, testSecret, 3))

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
		}
		if tu.gotTask.Title != "updated" || tu.gotUserId != 3 || tu.gotTaskId != 7 {
			t.Errorf("title, userId, taskId = %q, %d, %d", tu.gotTask.Title, tu.gotUserId, tu.gotTaskId)
		}
	})

	t.Run("DELETE /tasks/:taskId は 204", func(t *testing.T) {
		tu := &fakeTaskUsecase{}
		e := newTestServer(t, &fakeUserUsecase{}, tu)

		rec := doRequest(e, http.MethodDelete, "/tasks/7", "", signToken(t, testSecret, 3))

		if rec.Code != http.StatusNoContent {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusNoContent)
		}
		if tu.gotUserId != 3 || tu.gotTaskId != 7 {
			t.Errorf("userId, taskId = %d, %d, want 3, 7", tu.gotUserId, tu.gotTaskId)
		}
	})

	t.Run("usecase がエラーを返すと 500", func(t *testing.T) {
		reqs := []struct{ method, path, body string }{
			{http.MethodGet, "/tasks", ""},
			{http.MethodGet, "/tasks/7", ""},
			{http.MethodPost, "/tasks", `{"title":"x"}`},
			{http.MethodPut, "/tasks/7", `{"title":"x"}`},
			{http.MethodDelete, "/tasks/7", ""},
		}
		for _, r := range reqs {
			t.Run(r.method+" "+r.path, func(t *testing.T) {
				e := newTestServer(t, &fakeUserUsecase{}, &fakeTaskUsecase{err: errors.New("db down")})
				rec := doRequest(e, r.method, r.path, r.body, signToken(t, testSecret, 3))
				if rec.Code != http.StatusInternalServerError {
					t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
				}
			})
		}
	})
}
