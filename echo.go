package main

import (
	"fmt"
	"net/http"
	"time"

	"github.com/jinzhu/gorm"
	"github.com/labstack/echo"
)

type Result struct {
	Result bool `json:"result"`
}

type UserSignupRequest struct {
	UserId       string `json:"userId"`
	UserName     string `json:"userName"`
	UserPassword string `json:"userPassword"`
}

type UserLoginRequest struct {
	UserId       string `json:"userId"`
	UserPassword string `json:"userPassword"`
}

type UserLoginResult struct {
	Result   bool   `json:"result"`
	UserName string `json:"userName"`
}

type CreateMeetingRequest struct {
	MeetingName      string   `json:"meetingName"`
	MeetingStartTime string   `json:"meetingStartTime"`
	PresenterIds     []string `json:"presenterIds"`
}

type CreateMeetingResult struct {
	Result      bool   `json:"result"`
	MeetingId   int    `json:"meetingId"`
	MeetingName string `json:"meetingName"`
}

type JoinMeetingRequest struct {
	UserId    string `json:"userId"`
	MeetingId int    `json:"meetingId"`
}

type JoinMeetingResult struct {
	Result           bool     `json:"result"`
	MeetingName      string   `json:"meetingName"`
	MeetingStartTime string   `json:"meetingStartTime"`
	PresenterNames   []string `json:"presenterNames"`
	PresenterIds     []string `json:"presenterIds"`
	DocumentIds      []int    `json:"documentIds"`
}

type DocumentRegisterRequest struct {
	DocumentId  int    `json:"documentId"`
	DocumentUrl string `json:"documentUrl"`
	Script      string `json:"script"`
}

type DocumentRegisterResult struct {
	Result bool `json:"result"`
}

type DocumentGetRequest struct {
	DocumentId int `json:"documentId"`
}

type DocumentGetResult struct {
	Result      bool   `json:"result"`
	DocumentUrl string `json:"documentUrl"`
	Script      string `json:"script"`
}

var echo_startTime, echo_endTime time.Time

func initRouting(e *echo.Echo, hub *Hub, db *gorm.DB) {

	e.GET("/", func(c echo.Context) error {
		serveHome(c.Response(), c.Request())
		return nil
	})

	e.POST("/user/signup", func(c echo.Context) error {
		request := new(UserSignupRequest)
		err := c.Bind(request)
		if err == nil {
			result := &Result{
				Result: signupUser(db, request.UserId, request.UserName, request.UserPassword),
			}

			return c.JSON(http.StatusOK, result)
		} else {
			return c.JSON(http.StatusBadRequest, &Result{Result: false})
		}
	})

	e.POST("/user/login", func(c echo.Context) error {
		request := new(UserLoginRequest)
		err := c.Bind(request)
		if err == nil {
			resultLogin, userName := loginUser(db, request.UserId, request.UserPassword)
			result := &UserLoginResult{
				Result:   resultLogin,
				UserName: userName,
			}

			return c.JSON(http.StatusOK, result)
		} else {
			return c.JSON(http.StatusBadRequest, &UserLoginResult{Result: false, UserName: ""})
		}
	})

	e.POST("/meeting/join", func(c echo.Context) error {
		request := new(JoinMeetingRequest)
		err := c.Bind(request)
		if err == nil {
			resultJoinMeeting, meetingName, meetingStartTime, presenterNames, presenterIds, documentIds := joinMeeting(db, request.UserId, request.MeetingId)
			layout := "2006/01/02 15:04:05"
			meetingStartTimeString := meetingStartTime.Format(layout)
			result := &JoinMeetingResult{
				Result:           resultJoinMeeting,
				MeetingName:      meetingName,
				MeetingStartTime: meetingStartTimeString,
				PresenterNames:   presenterNames,
				PresenterIds:     presenterIds,
				DocumentIds:      documentIds,
			}
			if result.Result {
				go hub.sendStartMeetingMessage(request.MeetingId, meetingStartTime)
			}
			return c.JSON(http.StatusOK, result)
		} else {
			return c.JSON(http.StatusBadRequest, &Result{Result: false})
		}

	})

	e.GET("/ws", func(c echo.Context) error {
		serveWs(hub, c.Response(), c.Request())
		return nil
	})

	e.POST("/meeting/create", func(c echo.Context) error {
		request := new(CreateMeetingRequest)
		err := c.Bind(request)
		if err == nil {
			echo_startTime = time.Now()
			resultCreateMeeting, meetingId, meetingName := createMeeting(db, request.MeetingName, request.MeetingStartTime, request.PresenterIds)
			echo_endTime = time.Now()
			fmt.Println("createMeeting: took", echo_endTime.Sub(echo_startTime))
			result := &CreateMeetingResult{
				Result:      resultCreateMeeting,
				MeetingId:   meetingId,
				MeetingName: meetingName,
			}

			return c.JSON(http.StatusOK, result)
		} else {
			return c.JSON(http.StatusBadRequest, &Result{Result: false})
		}
	})

	e.POST("/document/register", func(c echo.Context) error {
		request := new(DocumentRegisterRequest)
		err := c.Bind(request)
		if err == nil {
			resultDocumentRegister := documentRegister(db, request.DocumentId, request.DocumentUrl, request.Script)
			result := &DocumentRegisterResult{
				Result: resultDocumentRegister,
			}
			return c.JSON(http.StatusOK, result)
		} else {
			return c.JSON(http.StatusBadRequest, &Result{Result: false})
		}
	})

	e.POST("/document/get", func(c echo.Context) error {
		request := new(DocumentGetRequest)
		err := c.Bind(request)
		if err == nil {
			resultDocumentGet, documentUrl, script := documentGet(db, request.DocumentId)
			result := &DocumentGetResult{
				Result:      resultDocumentGet,
				DocumentUrl: documentUrl,
				Script:      script,
			}
			return c.JSON(http.StatusOK, result)
		} else {
			return c.JSON(http.StatusBadRequest, &Result{Result: false})
		}
	})
}
