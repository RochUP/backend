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
	UserId       string //`gorm:"PRIMARY_KEY"`
	UserName     string //`json:"user_name"`
	UserPassword string //`json:"user_password"`
}

type Meeting struct {
	MeetingId        int       `gorm:"AUTO_INCREMENT"`
	MeetingName      string    //`json:"meeting_name`
	MeetingStartTime time.Time //`json:meeting_start_time`
	MeetingDone      bool      //`json:meeting_done`
}

type Participant struct {
	MeetingId        int    //`gorm:"PRIMARY_KEY"`
	UserId           string //`gorm:"PRIMARY_KEY"`
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
	IsVoice      bool
}

type Document struct {
	DocumentId  int `gorm:"AUTO_INCREMENT"`
	UserId      string
	MeetingId   int
	DocumentUrl *string
	Script      *string
}

type Reaction struct {
	DocumentId   int //`gorm:"PRIMARY_KEY"`
	DocumentPage int //`gorm:"PRIMARY_KEY"`
	ReactionNum  int
	SuggestionOk bool
}

type ByParticipantOrder []Participant

func (p ByParticipantOrder) Len() int           { return len(p) }
func (p ByParticipantOrder) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }
func (p ByParticipantOrder) Less(i, j int) bool { return p[i].ParticipantOrder < p[j].ParticipantOrder }

type ByQuestionTime []Question

func (q ByQuestionTime) Len() int           { return len(q) }
func (q ByQuestionTime) Swap(i, j int)      { q[i], q[j] = q[j], q[i] }
func (q ByQuestionTime) Less(i, j int) bool { return q[i].QuestionTime.Before(q[j].QuestionTime) }

type ReverseByReactionNum []Reaction

func (r ReverseByReactionNum) Len() int           { return len(r) }
func (r ReverseByReactionNum) Swap(i, j int)      { r[i], r[j] = r[j], r[i] }
func (r ReverseByReactionNum) Less(i, j int) bool { return r[i].ReactionNum > r[j].ReactionNum }

type BySpeakNum []Participant

func (p BySpeakNum) Len() int           { return len(p) }
func (p BySpeakNum) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }
func (p BySpeakNum) Less(i, j int) bool { return p[i].SpeakNum < p[j].SpeakNum }

var startTime, endTime time.Time

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
	startTime = time.Now()
	if err := db.Create(&user).Error; err == nil {
		endTime = time.Now()
		fmt.Println("Access to users: took", endTime.Sub(startTime))
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
	startTime = time.Now()
	err := db.First(&user, "user_id = ? AND user_password = ?", userId, userPassword).Error
	endTime = time.Now()
	fmt.Println("Access to users: took", endTime.Sub(startTime))
	if err == nil {
		fmt.Printf("login成功: %s, %s\n", userId, userPassword)
		return true, user.UserName
	} else {
		fmt.Println("login失敗")
		return false, ""
	}
}

func createMeeting(db *gorm.DB, meetingName string, startTimeStr string, presenterIds []string) (bool, int, string) {
	var (
		user         User
		layout       = "2006/01/02 15:04:05"
		location, _  = time.LoadLocation("Asia/Tokyo")
		startTime, _ = time.ParseInLocation(layout, startTimeStr, location)
		meeting      = Meeting{MeetingName: meetingName, MeetingStartTime: startTime, MeetingDone: false}
	)
	startTime = time.Now()
	if err := db.Create(&meeting).Error; err == nil {
		endTime = time.Now()
		fmt.Println("Access to meetings(Create): took", endTime.Sub(startTime))
		for i, presenter := range presenterIds {
			startTime = time.Now()
			if err := db.First(&user, "user_id = ?", presenter).Error; err == nil {
				endTime = time.Now()
				fmt.Println("Access to users(First): took", endTime.Sub(startTime))
				participant := Participant{MeetingId: meeting.MeetingId, UserId: user.UserId, SpeakNum: 0, ParticipantOrder: i}
				startTime = time.Now()
				if err := db.Create(&participant).Error; err == nil {
					endTime = time.Now()
					fmt.Println("Access to participants(Create): took", endTime.Sub(startTime))
					document := Document{UserId: user.UserId, MeetingId: meeting.MeetingId}
					startTime = time.Now()
					if err := db.Create(&document).Error; err != nil {
						fmt.Printf("create失敗(空の資料作成に失敗しました)\n")
						return false, -1, ""
					}
					endTime = time.Now()
					fmt.Println("Access to documents(Create): took", endTime.Sub(startTime))
				} else { // TODO: transaction
					fmt.Printf("create失敗(発表者%sの登録に失敗しました): %s, %s, %s\n", presenter, meetingName, startTimeStr, presenterIds)
					return false, -1, ""
				}
			} else {
				fmt.Printf("create失敗(発表者%sが見つかりません): %s, %s, %s\n", presenter, meetingName, startTimeStr, presenterIds)
				return false, -1, ""
			}
		}
		fmt.Printf("create成功: %s, %s, %s\n", meetingName, startTimeStr, presenterIds)
		return true, meeting.MeetingId, meeting.MeetingName
	} else {
		fmt.Printf("create失敗(会議の登録に失敗しました): %s, %s, %s\n", meetingName, startTimeStr, presenterIds)
		return false, -1, ""
	}
}

func joinMeeting(db *gorm.DB, userId string, meetingId int) (bool, string, time.Time, []string, []string, []int) {
	var user User
	var meeting Meeting
	var participant Participant
	var document Document
	participants := make([]Participant, 0, 10)
	startTime = time.Now()
	user_info := db.First(&user, "user_id = ?", userId)
	endTime = time.Now()
	fmt.Println("Access to users: took", endTime.Sub(startTime))
	startTime = time.Now()
	meeting_info := db.First(&meeting, "meeting_id = ?", meetingId)
	endTime = time.Now()
	fmt.Println("Access to meetings: took", endTime.Sub(startTime))
	if user_info.Error == nil && meeting_info.Error == nil {
		startTime = time.Now()
		participant_info := db.First(&participant, "user_id = ? AND meeting_id = ?", userId, meetingId)
		endTime = time.Now()
		fmt.Println("Access to participants: took", endTime.Sub(startTime))
		if participant_info.Error != nil {
			participant.MeetingId = meetingId
			participant.UserId = userId
			participant.SpeakNum = 0
			participant.ParticipantOrder = -1
			startTime = time.Now()
			if err := db.Create(&participant).Error; err == nil {
				endTime = time.Now()
				fmt.Println("Access to participants: took", endTime.Sub(startTime))
				fmt.Printf("参加者追加成功: %s, %d\n", userId, meetingId)
			} else {
				fmt.Println("参加者追加失敗")
				return false, "false", time.Now(), []string{}, []string{}, []int{}
			}
		}
		startTime = time.Now()
		if db.Find(&participants, "meeting_id = ? AND participant_order != -1", meetingId); len(participants) == 0 {
			endTime = time.Now()
			fmt.Println("Access to participants: took", endTime.Sub(startTime))
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
				startTime = time.Now()
				user_err := db.First(&user, "user_id = ?", presenter_id).Error
				endTime = time.Now()
				fmt.Println("Access to users: took", endTime.Sub(startTime))
				if user_err != nil {
					fmt.Println("ユーザー非存在")
					return false, "false", time.Now(), []string{}, []string{}, []int{}
				}
				startTime = time.Now()
				document_err := db.First(&document, "user_id = ? AND meeting_id = ?", p.UserId, p.MeetingId).Error
				endTime = time.Now()
				fmt.Println("Access to documents: took", endTime.Sub(startTime))
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

func documentRegister(db *gorm.DB, documentId int, documentUrl string, script string) bool {
	var document Document
	startTime = time.Now()
	if err := db.First(&document, "document_id = ?", documentId).Error; err != nil {
		endTime = time.Now()
		fmt.Println("Access to documents: took", endTime.Sub(startTime))
		fmt.Printf("資料が非存在: %d\n", documentId)
		return false
	}
	if documentUrl != "" {
		startTime = time.Now()
		if document_err := db.Model(&document).Where("document_id = ?", document.DocumentId).Update("document_url", documentUrl).Error; document_err != nil {
			fmt.Printf("update失敗(資料URLの登録に失敗しました): %d\n", document.DocumentId)
			return false
		} else {
			endTime = time.Now()
			fmt.Println("Access to documents: took", endTime.Sub(startTime))
			fmt.Printf("update成功(資料URLの登録に成功しました): %d\n", document.DocumentId)
		}
	}
	if script != "" {
		startTime = time.Now()
		if script_err := db.Model(&document).Where("document_id = ?", document.DocumentId).Update("script", script).Error; script_err != nil {
			fmt.Printf("update失敗(原稿の登録に失敗しました): %d\n", document.DocumentId)
			return false
		} else {
			endTime = time.Now()
			fmt.Println("Access to documents: took", endTime.Sub(startTime))
			fmt.Printf("update成功(原稿の登録に成功しました): %d\n", document.DocumentId)
		}
	}

	return true
}

func createQuestion(db *gorm.DB, question Question) (bool, int) {
	startTime = time.Now()
	if err := db.Create(&question).Error; err != nil {
		fmt.Printf("create失敗(質問の登録に失敗しました): %s, %d, %s\n", question.UserId, question.DocumentId, question.QuestionTime)
		return false, -1
	}
	endTime = time.Now()
	fmt.Println("Access to questions: took", endTime.Sub(startTime))
	fmt.Printf("create成功(質問の登録に成功しました): %s, %d, %s\n", question.UserId, question.DocumentId, question.QuestionTime)
	return true, question.QuestionId
}

func selectQuestion(db *gorm.DB, meetingId, documentId int, presenterId string) (bool, bool, string, int) {
	pickQuestioner := true
	suggestQuestion := false
	var question Question
	var participant Participant
	questionUserId := ""
	location, _ := time.LoadLocation("Asia/Tokyo")
	startTime = time.Now()
	if voice_question_err := db.First(&question, "document_id = ? AND question_ok = ? AND is_voice = ?", documentId, false, true).Error; voice_question_err == nil {
		endTime = time.Now()
		fmt.Println("Access to questions: took", endTime.Sub(startTime))
		startTime = time.Now()
		if question_err := db.Model(&question).Where("question_id = ?", question.QuestionId).Update("question_ok", true).Error; question_err != nil {
			fmt.Printf("update失敗(質問の回答状況の更新に失敗しました): %d\n", question.QuestionId)
			return false, false, "", -1
		}
		endTime = time.Now()
		fmt.Println("Access to questions: took", endTime.Sub(startTime))
		startTime = time.Now()
		if incSpeakNum_err := db.Model(&participant).Where("meeting_id = ? AND user_id = ?", meetingId, question.UserId).Update("speak_num", participant.SpeakNum+1).Error; incSpeakNum_err != nil {
			fmt.Printf("update失敗(参加者の話数の更新に失敗しました): %s, %d, %d\n", participant.UserId, participant.MeetingId, participant.SpeakNum)
			return false, false, "", -1
		}
		endTime = time.Now()
		fmt.Println("Access to participants: took", endTime.Sub(startTime))
		questionUserId = question.UserId
		return pickQuestioner, suggestQuestion, questionUserId, question.QuestionId
	} else {
		startTime = time.Now()
		if not_voice_question_err := db.First(&question, "document_id = ? AND question_ok = ? AND is_voice = ?", documentId, false, false).Error; not_voice_question_err == nil {
			endTime = time.Now()
			fmt.Println("Access to questions: took", endTime.Sub(startTime))
			startTime = time.Now()
			if question_err := db.Model(&question).Where("question_id = ?", question.QuestionId).Update("question_ok", true).Error; question_err != nil {
				fmt.Printf("update失敗(質問の回答状況の更新に失敗しました): %d\n", question.QuestionId)
				return false, false, "", -1
			}
			endTime = time.Now()
			fmt.Println("Access to questions: took", endTime.Sub(startTime))
			pickQuestioner = false
			startTime = time.Now()
			if incSpeakNum_err := db.Model(&participant).Where("meeting_id = ? AND user_id = ?", meetingId, question.UserId).Update("speak_num", participant.SpeakNum+1).Error; incSpeakNum_err != nil {
				fmt.Printf("update失敗(参加者の話数の更新に失敗しました): %s, %d, %d\n", participant.UserId, participant.MeetingId, participant.SpeakNum)
				return false, false, "", -1
			}
			endTime = time.Now()
			fmt.Println("Access to participants: took", endTime.Sub(startTime))
		}
	}
	if pickQuestioner {
		participants := make([]Participant, 0, 10)
		startTime = time.Now()
		if db.Find(&participants, "meeting_id = ? AND user_id != ?", meetingId, presenterId); len(participants) != 0 {
			endTime = time.Now()
			fmt.Println("Access to participants: took", endTime.Sub(startTime))
			reactions := make([]Reaction, 0, 10)
			startTime = time.Now()
			if db.Find(&reactions, "document_id = ? AND suggestion_ok = ?", documentId, false); len(reactions) != 0 {
				endTime = time.Now()
				fmt.Println("Access to reactions: took", endTime.Sub(startTime))
				sort.Sort(ReverseByReactionNum(reactions))
				if reactions[0].ReactionNum >= len(participants)/2 {
					startTime = time.Now()
					if reaction_err := db.Model(&reactions[0]).Where("document_id = ? AND document_page = ?", reactions[0].DocumentId, reactions[0].DocumentPage).Update("suggestion_ok", true).Error; reaction_err != nil {
						fmt.Printf("update失敗(資料リアクションの提案状況の更新に失敗しました): %d, %d\n", reactions[0].DocumentId, reactions[0].DocumentPage)
						return false, false, "", -1
					}
					endTime = time.Now()
					fmt.Println("Access to reactions: took", endTime.Sub(startTime))
					question = Question{
						UserId:       "Moderator",
						QuestionBody: fmt.Sprintf("%dページについての詳しい説明を要求．", reactions[0].DocumentPage),
						DocumentId:   reactions[0].DocumentId,
						DocumentPage: reactions[0].DocumentPage,
						VoteNum:      reactions[0].ReactionNum,
						QuestionTime: time.Now().In(location),
						QuestionOk:   true,
						IsVoice:      false,
					}
					startTime = time.Now()
					if err := db.Create(&question).Error; err != nil {
						fmt.Printf("create失敗(質問の登録に失敗しました): %s, %d, %s\n", question.UserId, question.DocumentId, question.QuestionTime)
						return false, false, "", -1
					}
					endTime = time.Now()
					fmt.Println("Access to questions: took", endTime.Sub(startTime))
					fmt.Printf("create成功(質問の登録に成功しました): %s, %d, %s\n", question.UserId, question.DocumentId, question.QuestionTime)
					pickQuestioner = false
					suggestQuestion = true
					return pickQuestioner, suggestQuestion, questionUserId, question.QuestionId
				}
			}
			sort.Sort(BySpeakNum(participants))
			rand_max := 3
			if len(participants) < 3 {
				rand_max = len(participants)
			}
			participant = participants[rand.Intn(rand_max)]
			questionUserId = participant.UserId
			question = Question{
				UserId:       questionUserId,
				QuestionBody: "",
				DocumentId:   documentId,
				DocumentPage: -1,
				VoteNum:      0,
				QuestionTime: time.Now().In(location),
				QuestionOk:   true,
				IsVoice:      true,
			}
			startTime = time.Now()
			if err := db.Create(&question).Error; err != nil {
				fmt.Printf("create失敗(質問の登録に失敗しました): %s, %d, %s\n", question.UserId, question.DocumentId, question.QuestionTime)
				return false, false, "", -1
			}
			endTime = time.Now()
			fmt.Println("Access to questions: took", endTime.Sub(startTime))
			startTime = time.Now()
			if err := db.Model(&participant).Where("user_id = ? AND meeting_id = ?", participant.UserId, participant.MeetingId).Update("speak_num", participant.SpeakNum+1).Error; err != nil {
				fmt.Printf("update失敗(参加者の話数の更新に失敗しました): %s, %d, %d\n", participant.UserId, participant.MeetingId, participant.SpeakNum)
				return false, false, "", -1
			}
			endTime = time.Now()
			fmt.Println("Access to participants: took", endTime.Sub(startTime))
			fmt.Printf("create成功(質問の登録に成功しました): %s, %d, %s\n", question.UserId, question.DocumentId, question.QuestionTime)

		} else {
			fmt.Printf("参加者が非存在: %d\n", meetingId)
			return false, false, "", -1
		}
	}
	return pickQuestioner, suggestQuestion, questionUserId, question.QuestionId
}

func voteQuestion(db *gorm.DB, questionId int, isVote bool) (int, int, int) {
	var question Question
	var document Document

	startTime = time.Now()
	if err := db.First(&question, "question_id = ?", questionId).Error; err != nil {
		fmt.Printf("質問が非存在: %d\n", questionId)
		return -1, -1, -1
	}
	endTime = time.Now()
	fmt.Println("Access to vote_question_view(first): took", endTime.Sub(startTime))
	voteNum := question.VoteNum
	if isVote {
		voteNum += 1
	} else {
		voteNum -= 1
	}
	startTime = time.Now()
	if err := db.Model(&question).Where("question_id = ?", questionId).Update("vote_num", voteNum).Error; err != nil {
		fmt.Printf("update失敗(質問の投票数の更新に失敗しました): %d\n", voteNum)
		return -1, -1, -1
	}
	endTime = time.Now()
	fmt.Println("Access to vote_question_view(update): took", endTime.Sub(startTime))
	startTime = time.Now()
	if err := db.First(&document, "document_id = ?", question.DocumentId).Error; err != nil {
		fmt.Printf("資料が非存在: %d\n", question.DocumentId)
		return -1, -1, -1
	}
	endTime = time.Now()
	fmt.Println("Access to documents: took", endTime.Sub(startTime))

	return document.MeetingId, questionId, voteNum
}

func handsUp(db *gorm.DB, userId string, documentId int, documentPage int) int {
	var document Document
	location, _ := time.LoadLocation("Asia/Tokyo")
	startTime = time.Now()
	if document_err := db.First(&document, "document_id = ?", documentId).Error; document_err != nil {
		fmt.Printf("資料が非存在: %d\n", documentId)
		return -1
	}
	endTime = time.Now()
	fmt.Println("Access to documents: took", endTime.Sub(startTime))
	startTime = time.Now()
	if user_err := db.First(&User{}, "user_id = ?", userId).Error; user_err != nil {
		fmt.Printf("ユーザーが非存在: %s\n", userId)
		return -1
	}
	endTime = time.Now()
	fmt.Println("Access to users: took", endTime.Sub(startTime))

	question := Question{
		UserId:       userId,
		QuestionBody: "",
		DocumentId:   document.DocumentId,
		DocumentPage: documentPage,
		VoteNum:      0,
		QuestionTime: time.Now().In(location),
		QuestionOk:   false,
		IsVoice:      true,
	}
	startTime = time.Now()
	if question_err := db.Create(&question).Error; question_err != nil {
		fmt.Printf("create失敗(質問の登録に失敗しました): %s, %d, %d, %s\n", question.UserId, question.DocumentId, question.DocumentPage, question.QuestionTime)
		return -1
	}
	endTime = time.Now()
	fmt.Println("Access to questions: took", endTime.Sub(startTime))
	fmt.Printf("create成功(質問の登録に成功しました): %s, %d, %d, %s\n", question.UserId, question.DocumentId, question.DocumentPage, question.QuestionTime)
	return document.MeetingId
}

func handsDown(db *gorm.DB, userId string, documentId int, documentPage int) int {
	var (
		document Document
		question Question
	)
	startTime = time.Now()
	if document_err := db.First(&document, "document_id = ?", documentId).Error; document_err != nil {
		fmt.Printf("資料が非存在: %d\n", documentId)
		return -1
	}
	endTime = time.Now()
	fmt.Println("Access to documents: took", endTime.Sub(startTime))
	startTime = time.Now()
	if user_err := db.First(&User{}, "user_id = ?", userId).Error; user_err != nil {
		fmt.Printf("ユーザーが非存在: %s\n", userId)
		return -1
	}
	endTime = time.Now()
	fmt.Println("Access to users: took", endTime.Sub(startTime))
	startTime = time.Now()
	if question_err := db.First(&question, "user_id = ? AND document_id = ? AND document_page = ? AND question_ok = ? AND is_voice = ?", userId, document.DocumentId, documentPage, false, true).Error; question_err != nil {
		fmt.Printf("質問が非存在: %s, %d, %d\n", userId, document.DocumentId, documentPage)
		return -1
	}
	endTime = time.Now()
	fmt.Println("Access to questions: took", endTime.Sub(startTime))
	startTime = time.Now()
	if delete_question_err := db.Where("question_id = ?", question.QuestionId).Delete(&question).Error; delete_question_err != nil {
		fmt.Printf("delete失敗(質問の削除に失敗しました): %d\n", question.QuestionId)
		return -1
	}
	endTime = time.Now()
	fmt.Println("Access to questions: took", endTime.Sub(startTime))
	fmt.Printf("delete成功(質問の削除に成功しました): %d\n", question.QuestionId)
	return document.MeetingId
}

func voteReaction(db *gorm.DB, documentId int, documentPage int, isReaction bool) (int, int) {
	var document Document
	var reaction Reaction
	startTime = time.Now()
	if document_err := db.First(&document, "document_id = ?", documentId).Error; document_err != nil {
		fmt.Printf("資料が非存在: %d\n", documentId)
		return -1, -1
	}
	endTime = time.Now()
	fmt.Println("Access to documents: took", endTime.Sub(startTime))
	startTime = time.Now()
	if reaction_err := db.First(&reaction, "document_id = ? AND document_page = ?", documentId, documentPage).Error; reaction_err != nil {
		if !isReaction {
			fmt.Printf("資料リアクションが非存在: %d, %d\n", documentId, documentPage)
			return -1, -1
		}
		endTime = time.Now()
		fmt.Println("Access to reactions: took", endTime.Sub(startTime))
		reaction = Reaction{
			DocumentId:   document.DocumentId,
			DocumentPage: documentPage,
			ReactionNum:  1,
			SuggestionOk: false,
		}
		startTime = time.Now()
		if create_reaction_err := db.Create(&reaction).Error; create_reaction_err != nil {
			fmt.Printf("create失敗(資料リアクションの登録に失敗しました): %d, %d\n", reaction.DocumentId, reaction.DocumentPage)
			return -1, -1
		}
		endTime = time.Now()
		fmt.Println("Access to reactions: took", endTime.Sub(startTime))
		fmt.Printf("create成功(資料リアクションの登録に成功しました): %d, %d\n", reaction.DocumentId, reaction.DocumentPage)
	} else {
		endTime = time.Now()
		fmt.Println("Access to reactions: took", endTime.Sub(startTime))
		reactionNum := reaction.ReactionNum
		if isReaction {
			reactionNum += 1
		} else {
			reactionNum -= 1
		}
		startTime = time.Now()
		if update_reaction_num_err := db.Model(&reaction).Where("document_id = ? AND document_page = ?", reaction.DocumentId, reaction.DocumentPage).Update("reaction_num", reactionNum).Error; update_reaction_num_err != nil {
			fmt.Printf("update失敗(資料リアクションのリアクション数の更新に失敗しました): %d, %d\n", reaction.DocumentId, reaction.DocumentPage)
			return -1, -1
		}
		endTime = time.Now()
		fmt.Println("Access to reactions: took", endTime.Sub(startTime))
		fmt.Printf("update成功(資料リアクションのリアクション数の更新に成功しました): %d, %d\n", reaction.DocumentId, reaction.DocumentPage)
	}
	return document.MeetingId, reaction.ReactionNum
}

func getNextPresenterId(db *gorm.DB, meetingId int, nowPresenterId string) (bool, string, int) {
	var participant Participant
	startTime = time.Now()
	if participant_err := db.First(&participant, "meeting_id = ? AND user_id = ?", meetingId, nowPresenterId).Error; participant_err != nil {
		fmt.Printf("参加者が非存在: %s\n", nowPresenterId)
		return false, "", -1
	}
	endTime = time.Now()
	fmt.Println("Access to participants: took", endTime.Sub(startTime))
	nextOrder := participant.ParticipantOrder + 1
	startTime = time.Now()
	if meeting_end_err := db.First(&participant, "meeting_id = ? AND participant_order = ?", meetingId, nextOrder).Error; meeting_end_err != nil {
		endTime = time.Now()
		fmt.Println("Access to participants: took", endTime.Sub(startTime))
		fmt.Printf("会議終了につき次の発表者が非存在: %d\n", nextOrder)
		return true, "", -1
	}
	endTime = time.Now()
	fmt.Println("Access to participants: took", endTime.Sub(startTime))
	return false, participant.UserId, nextOrder
}

func getUserName(db *gorm.DB, userId string) string {
	var user User
	startTime = time.Now()
	if err := db.First(&user, "user_id = ?", userId).Error; err != nil {
		fmt.Printf("ユーザーが非存在: %s\n", userId)
		return ""
	}
	endTime = time.Now()
	fmt.Println("Access to users: took", endTime.Sub(startTime))
	return user.UserName
}

func getQuestionBody(db *gorm.DB, questionId int) (string, int) {
	var question Question
	startTime = time.Now()
	if err := db.First(&question, "question_id = ?", questionId).Error; err != nil {
		fmt.Printf("質問が非存在: %d\n", questionId)
		return "", -1
	}
	endTime = time.Now()
	fmt.Println("Access to documents: took", endTime.Sub(startTime))
	return question.QuestionBody, question.DocumentPage
}

func getQuestionDocumentPage(db *gorm.DB, questionId int) int {
	var question Question
	startTime = time.Now()
	if err := db.First(&question, "question_id = ?", questionId).Error; err != nil {
		fmt.Printf("質問が非存在: %d\n", questionId)
		return -1
	}
	endTime = time.Now()
	fmt.Println("Access to questions: took", endTime.Sub(startTime))
	return question.DocumentPage
}

func getDocumentId(db *gorm.DB, userId string, meetingId int) int {
	var document Document
	startTime = time.Now()
	if err := db.First(&document, "user_id = ? AND meeting_id = ?", userId, meetingId).Error; err != nil {
		fmt.Printf("資料が非存在: %s, %d\n", userId, meetingId)
		return -1
	}
	endTime = time.Now()
	fmt.Println("Access to documents: took", endTime.Sub(startTime))
	return document.DocumentId
}

func documentGet(db *gorm.DB, documentId int) (bool, string, string) {
	var (
		document    Document
		documentUrl *string
		script      *string
		emptyString = ""
	)
	startTime = time.Now()
	if err := db.First(&document, "document_id = ?", documentId).Error; err != nil {
		fmt.Printf("資料が非存在: %d\n", documentId)
		return false, "", ""
	}
	endTime = time.Now()
	fmt.Println("Access to documents: took", endTime.Sub(startTime))
	if documentUrl = document.DocumentUrl; documentUrl == nil {
		fmt.Printf("資料URLが非存在: %d\n", documentId)
		documentUrl = &emptyString
	}
	if script = document.Script; script == nil {
		fmt.Printf("原稿が非存在: %d\n", documentId)
		script = &emptyString
	}
	return true, *documentUrl, *script
}

func getPresenterId(db *gorm.DB, documentId int) string {
	var document Document
	startTime = time.Now()
	if err := db.First(&document, "document_id = ?", documentId).Error; err != nil {
		fmt.Printf("資料が非存在: %d\n", documentId)
		return ""
	}
	endTime = time.Now()
	fmt.Println("Access to documents: took", endTime.Sub(startTime))
	return document.UserId
}

func setMeetingDone(db *gorm.DB, meetingId int) {
	var meeting Meeting
	startTime = time.Now()
	db.First(&meeting, "meeting_id = ?", meetingId)
	endTime = time.Now()
	fmt.Println("Access to meetings: took", endTime.Sub(startTime))
	startTime = time.Now()
	db.Model(&meeting).Where("meeting_id = ?", meetingId).Update("meeting_done", true)
	endTime = time.Now()
	fmt.Println("Access to meetings: took", endTime.Sub(startTime))
}
