package main

import (
	"fmt"
	"math/rand"
	"os"
	"sort"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/jinzhu/gorm"
)

type User struct {
	UserId       string //`json:"user_id"`
	UserName     string //`json:"user_name"`
	UserPassword string //`json:"user_password"`
}

type Meeting struct {
	MeetingId        int    `gorm:"AUTO_INCREMENT"`
	MeetingName      string //`json:"meeting_name`
	MeetingStartTime time.Time
}

type Participant struct {
	MeetingId        int    //`json:"meeting_id"`
	UserId           string //`json:"user_id"`
	SpeakNum         int    //`json:"speaknum"`
	ParticipantOrder int    //`json:"participantorder"`
}

type Question struct {
	QuestionId   int `gorm:"AUTO_INCREMENT"`
	UserId       string
	QuestionBody string
	DocumentId   int
	DocumentPage int
	VoteNum      int
	QuestionTime time.Time
	QuestionOk   bool
}

type Document struct {
	DocumentId  int `gorm:"AUTO_INCREMENT"`
	UserId      string
	MeetingId   int
	DocumentUrl *string
	script      *string
}

type ByParticipantOrder []Participant

func (p ByParticipantOrder) Len() int           { return len(p) }
func (p ByParticipantOrder) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }
func (p ByParticipantOrder) Less(i, j int) bool { return p[i].ParticipantOrder < p[j].ParticipantOrder }

type ByQuestionTime []Question

func (q ByQuestionTime) Len() int           { return len(q) }
func (q ByQuestionTime) Swap(i, j int)      { q[i], q[j] = q[j], q[i] }
func (q ByQuestionTime) Less(i, j int) bool { return q[i].QuestionTime.Before(q[j].QuestionTime) }

type ReverseBySpeakNum []Participant

func (p ReverseBySpeakNum) Len() int           { return len(p) }
func (p ReverseBySpeakNum) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }
func (p ReverseBySpeakNum) Less(i, j int) bool { return p[i].SpeakNum > p[j].SpeakNum }

// SQLConnect DB接続
func sqlConnect() (database *gorm.DB, err error) {
	DBMS := os.Getenv("DBMS")
	USER := os.Getenv("USER")
	PASS := os.Getenv("PASS")
	PROTOCOL := os.Getenv("PROTOCOL")
	DBNAME := os.Getenv("DBNAME")

	CONNECT := USER + ":" + PASS + "@" + PROTOCOL + "/" + DBNAME + "?charset=utf8&parseTime=true&loc=Asia%2FTokyo"
	return gorm.Open(DBMS, CONNECT)
}

func connectDB() *gorm.DB {
	// DB接続
	db, err := sqlConnect()
	if err != nil {
		panic(err.Error())
	} else {
		fmt.Println("DBへの接続に成功しました")
	}

	return db
}

func signupUser(db *gorm.DB, userId string, userName string, userPassword string) bool {
	user := User{UserId: userId, UserName: userName, UserPassword: userPassword}
	if err := db.Create(&user).Error; err == nil {
		fmt.Printf("signup成功: %s, %s, %s\n", userId, userName, userPassword)
		return true
	} else {
		fmt.Println("signup失敗")
		return false
	}
}

func loginUser(db *gorm.DB, userId string, userPassword string) (bool, string) {
	var user User
	// err := db.Find(&user).Error
	err := db.First(&user, "user_id = ? AND user_password = ?", userId, userPassword).Error
	if err == nil {
		fmt.Printf("login成功: %s, %s\n", userId, userPassword)
		return true, user.UserName
	} else {
		fmt.Println("login失敗")
		return false, ""
	}
}

func createMeeting(db *gorm.DB, meetingName string, startTimeStr string, presenterIds []string) (bool, int) {
	var (
		user         User
		documentIds  []int
		layout       = "2006/01/02 15:04:05"
		location, _  = time.LoadLocation("Asia/Tokyo")
		startTime, _ = time.ParseInLocation(layout, startTimeStr, location)
		meeting      = Meeting{MeetingName: meetingName, MeetingStartTime: startTime}
	)

	if err := db.Create(&meeting).Error; err == nil {
		for i, presenter := range presenterIds {
			if err := db.First(&user, "user_id = ?", presenter).Error; err == nil {
				participant := Participant{MeetingId: meeting.MeetingId, UserId: user.UserId, SpeakNum: 0, ParticipantOrder: i}
				if err := db.Create(&participant).Error; err == nil {
					document := Document{UserId: user.UserId, MeetingId: meeting.MeetingId}
					if err := db.Create(&document).Error; err == nil {
						documentIds = append(documentIds, document.DocumentId)
					} else {
						fmt.Printf("create失敗(空の資料作成に失敗しました)\n")
						return false, -1
					}
				} else { // TODO: transaction
					fmt.Printf("create失敗(発表者%sの登録に失敗しました): %s, %s, %s\n", presenter, meetingName, startTimeStr, presenterIds)
					return false, -1
				}
			} else {
				fmt.Printf("create失敗(発表者%sが見つかりません): %s, %s, %s\n", presenter, meetingName, startTimeStr, presenterIds)
				return false, -1
			}
		}
		fmt.Printf("create成功: %s, %s, %s\n", meetingName, startTimeStr, presenterIds)
		return true, meeting.MeetingId
	} else {
		fmt.Printf("create失敗(会議の登録に失敗しました): %s, %s, %s\n", meetingName, startTimeStr, presenterIds)
		return false, -1
	}
}

func joinMeeting(db *gorm.DB, userId string, meetingId int) (bool, string, time.Time, []string, []string, []int) {
	var user User
	var meeting Meeting
	var participant Participant
	var document Document
	participants := make([]Participant, 0, 10)
	user_info := db.First(&user, "user_id = ?", userId)
	meeting_info := db.First(&meeting, "meeting_id = ?", meetingId)
	if user_info.Error == nil && meeting_info.Error == nil {
		participant_info := db.First(&participant, "user_id = ? AND meeting_id = ?", userId, meetingId)
		if participant_info.Error != nil {
			participant.MeetingId = meetingId
			participant.UserId = userId
			participant.SpeakNum = 0
			participant.ParticipantOrder = -1
			if err := db.Create(&participant).Error; err == nil {
				fmt.Printf("参加者追加成功: %s, %d\n", userId, meetingId)
			} else {
				fmt.Println("参加者追加失敗")
				return false, "false", time.Now(), []string{}, []string{}, []int{}
			}
		}
		if db.Find(&participants, "meeting_id = ? AND participant_order != -1", meetingId); len(participants) == 0 {
			fmt.Println("会議非存在")
			return false, "false", time.Now(), []string{}, []string{}, []int{}
		}
		presenter_names := make([]string, 0, 10)
		presenter_ids := make([]string, 0, 10)
		document_ids := make([]int, 0, 10)

		sort.Sort(ByParticipantOrder(participants))

		for _, p := range participants {
			if p.ParticipantOrder != -1 {
				presenter_id := p.UserId
				user_err := db.First(&user, "user_id = ?", presenter_id).Error
				if user_err != nil {
					fmt.Println("ユーザー非存在")
					return false, "false", time.Now(), []string{}, []string{}, []int{}
				}
				document_err := db.First(&document, "user_id = ? AND meeting_id = ?", p.UserId, p.MeetingId).Error
				if document_err != nil {
					fmt.Println("資料非存在")
					return false, "false", time.Now(), []string{}, []string{}, []int{}
				}
				presenter_names = append(presenter_names, user.UserName)
				presenter_ids = append(presenter_ids, user.UserId)
				document_ids = append(document_ids, document.DocumentId)
			}
		}

		fmt.Printf("join成功: %s, %d\n", userId, meetingId)
		return true, meeting.MeetingName, meeting.MeetingStartTime, presenter_names, presenter_ids, document_ids

	} else {
		fmt.Println("ユーザーもしくは会議が非存在")
		return false, "false", time.Now(), []string{}, []string{}, []int{}
	}
}

func createQuestion(db *gorm.DB, question Question) (bool, int) {
	if err := db.Create(&question).Error; err != nil {
		fmt.Printf("create失敗(質問の登録に失敗しました): %s, %d, %s\n", question.UserId, question.DocumentId, question.QuestionTime)
		return false, -1
	}
	fmt.Printf("create成功(質問の登録に成功しました): %s, %d, %s\n", question.UserId, question.DocumentId, question.QuestionTime)
	return true, question.QuestionId
}

func selectQuestion(db *gorm.DB, meetingId, documentId int, presenterId string) (bool, string, int) {
	isUserId := true
	questions := make([]Question, 0, 10)
	question_user_id := ""
	question_id := -1
	if db.Find(&questions, "document_id = ?", documentId); len(questions) != 0 {
		sort.Sort(ByQuestionTime(questions))
		for _, q := range questions {
			if !q.QuestionOk {
				if q_err := db.Model(&q).Update("question_ok", true).Error; q_err != nil {
					fmt.Printf("update失敗(質問の回答状況の更新に失敗しました): %d\n", q.QuestionId)
					return false, "", -1
				}
				question_id = q.QuestionId
				isUserId = false
				break
			}
		}
	}
	if isUserId {
		participants := make([]Participant, 0, 10)
		if db.Find(&participants, "meeting_id = ? AND user_id != ?", meetingId, presenterId); len(participants) != 0 {
			sort.Sort(ReverseBySpeakNum(participants))
			rand_max := 3
			if len(participants) < 3 {
				rand_max = len(participants)
			}
			question_user_id = participants[rand.Intn(rand_max)].UserId
		} else {
			fmt.Printf("参加者が非存在: %d\n", meetingId)
			return false, "", -1
		}
	}
	return isUserId, question_user_id, question_id
}

func getNextPresenterId(db *gorm.DB, meetingId int, nowPresenterId string) (bool, string) {
	var participant Participant
	if err := db.First(&participant, "meeting_id = ? AND user_id = ?", meetingId, nowPresenterId).Error; err != nil {
		fmt.Printf("参加者が非存在: %s\n", nowPresenterId)
		return false, ""
	}
	nextOrder := participant.ParticipantOrder + 1
	if err := db.First(&participant, "meeting_id = ? AND participant_order = ?", meetingId, nextOrder).Error; err != nil {
		fmt.Printf("次の発表者が非存在: %d\n", nextOrder)
		return true, ""
	}
	return false, participant.UserId
}

func getUserName(db *gorm.DB, userId string) string {
	var user User
	if err := db.First(&user, "user_id = ?", userId).Error; err != nil {
		fmt.Printf("ユーザーが非存在: %s\n", userId)
		return ""
	}
	return user.UserName
}

func getQuestionBody(db *gorm.DB, questionId int) (string, int) {
	var question Question
	if err := db.First(&question, "question_id = ?", questionId).Error; err != nil {
		fmt.Printf("質問が非存在: %d\n", questionId)
		return "", -1
	}
	return question.QuestionBody, question.DocumentPage
}
