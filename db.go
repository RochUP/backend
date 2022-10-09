package main

import (
	"fmt"
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
	IsJoining        bool
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

type QuestionAndPresenterId struct {
	QuestionId   int
	QuestionBody string
	DocumentId   int
	DocumentPage int
	QuestionTime time.Time
	UserId       string
	VoteNum      int
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

type ReverseByVoteNum []Question

func (q ReverseByVoteNum) Len() int           { return len(q) }
func (q ReverseByVoteNum) Swap(i, j int)      { q[i], q[j] = q[j], q[i] }
func (q ReverseByVoteNum) Less(i, j int) bool { return q[i].VoteNum > q[j].VoteNum }

type BySpeakNum []Participant

func (p BySpeakNum) Len() int           { return len(p) }
func (p BySpeakNum) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }
func (p BySpeakNum) Less(i, j int) bool { return p[i].SpeakNum < p[j].SpeakNum }

// SQLConnect DB接続
func sqlConnect() (database *gorm.DB, err error) {
	DBMS := os.Getenv("DBMS")
	DBUSER := os.Getenv("DBUSER")
	DBPASS := os.Getenv("DBPASS")
	DBPROTOCOL := os.Getenv("DBPROTOCOL")
	DBNAME := os.Getenv("DBNAME")

	CONNECT := DBUSER + ":" + DBPASS + "@" + DBPROTOCOL + "/" + DBNAME + "?tls=true&charset=utf8&parseTime=true&loc=Asia%2FTokyo"
	return gorm.Open(DBMS, CONNECT)
}

func connectDB() *gorm.DB {
	// DB接続
	db, err := sqlConnect()
	if err != nil {
		panic(err.Error())
	} else {
		fmt.Printf("Log: DBへの接続に成功しました in connectDB\n")
	}

	return db
}

func signupUser(db *gorm.DB, userId string, userName string, userPassword string) bool {
	user := User{UserId: userId, UserName: userName, UserPassword: userPassword}
	if err := db.Create(&user).Error; err == nil {
		fmt.Printf("Log: signup成功: %s, %s, %s in signupUser\n", userId, userName, userPassword)
		return true
	} else {
		fmt.Printf("Error: signup失敗: %s, %s, %s in signupUser\n", userId, userName, userPassword)
		return false
	}
}

func loginUser(db *gorm.DB, userId string, userPassword string) (bool, string) {
	var user User
	// err := db.Find(&user).Error
	err := db.First(&user, "user_id = ? AND user_password = ?", userId, userPassword).Error
	if err == nil {
		fmt.Printf("Log: login成功: %s, %s in loginUser\n", userId, userPassword)
		return true, user.UserName
	} else {
		fmt.Printf("Error: login失敗: %s, %s in loginUser\n", userId, userPassword)
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

	if err := db.Create(&meeting).Error; err == nil {
		for i, presenter := range presenterIds {
			if err := db.First(&user, "user_id = ?", presenter).Error; err == nil {
				participant := Participant{MeetingId: meeting.MeetingId, UserId: user.UserId, SpeakNum: 0, ParticipantOrder: i, IsJoining: false}
				if err := db.Create(&participant).Error; err == nil {
					document := Document{UserId: user.UserId, MeetingId: meeting.MeetingId}
					if err := db.Create(&document).Error; err != nil {
						fmt.Printf("Error: create失敗(空の資料作成に失敗しました): %s, %d in createMeeting\n", user.UserId, meeting.MeetingId)
						return false, -1, ""
					}
				} else { // TODO: transaction
					fmt.Printf("Error: create失敗(発表者%sの登録に失敗しました): %s, %s, %s in createMeeting\n", presenter, meetingName, startTimeStr, presenterIds)
					return false, -1, ""
				}
			} else {
				fmt.Printf("Error: create失敗(発表者%sが見つかりません): %s, %s, %s in createMeeting\n", presenter, meetingName, startTimeStr, presenterIds)
				return false, -1, ""
			}
		}
		fmt.Printf("Log: create成功: %s, %s, %s in createMeeting\n", meetingName, startTimeStr, presenterIds)
		return true, meeting.MeetingId, meeting.MeetingName
	} else {
		fmt.Printf("Error: create失敗(会議の登録に失敗しました): %s, %s, %s in createMeeting\n", meetingName, startTimeStr, presenterIds)
		return false, -1, ""
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
		participant_info := db.First(&participant, "meeting_id = ? AND user_id = ?", meetingId, userId).Where(&participant, "meeting_id = ? AND user_id = ?", meetingId, userId).Update("is_joining", true)
		if participant_info.Error != nil {
			participant.MeetingId = meetingId
			participant.UserId = userId
			participant.SpeakNum = 0
			participant.ParticipantOrder = -1
			participant.IsJoining = true
			if err := db.Create(&participant).Error; err == nil {
				fmt.Printf("Log: 参加者追加成功: %s, %d in joinMeeting\n", userId, meetingId)
			} else {
				fmt.Printf("Error: 参加者追加失敗: %s, %d in joinMeeting\n", userId, meetingId)
				return false, "false", time.Now(), []string{}, []string{}, []int{}
			}
		} else {
			if participant_err := db.Model(&participant).Where("meeting_id = ? AND user_id = ?", meetingId, userId).Update("is_joining", true).Error; participant_err != nil {
				fmt.Printf("Error: update失敗(参加者の参加状態の更新に失敗しました): %s, %d in joinMeeting\n", userId, meetingId)
				return false, "false", time.Now(), []string{}, []string{}, []int{}
			}
		}
		if db.Find(&participants, "meeting_id = ? AND participant_order != ?", meetingId, -1); len(participants) == 0 {
			fmt.Printf("Error: 発表者非存在: %d in joinMeeting\n", meetingId)
			return false, "false", time.Now(), []string{}, []string{}, []int{}
		}
		presenter_names := make([]string, 0, 10)
		presenter_ids := make([]string, 0, 10)
		document_ids := make([]int, 0, 10)

		sort.Sort(ByParticipantOrder(participants))

		for _, p := range participants {
			presenter_id := p.UserId
			user_err := db.First(&user, "user_id = ?", presenter_id).Error
			if user_err != nil {
				fmt.Printf("Error: ユーザー非存在: %s in joinMeeting\n", presenter_id)
				return false, "false", time.Now(), []string{}, []string{}, []int{}
			}
			document_err := db.First(&document, "user_id = ? AND meeting_id = ?", p.UserId, p.MeetingId).Error
			if document_err != nil {
				fmt.Printf("Error: 資料非存在: %s, %d in joinMeeting\n", p.UserId, p.MeetingId)
				return false, "false", time.Now(), []string{}, []string{}, []int{}
			}
			presenter_names = append(presenter_names, user.UserName)
			presenter_ids = append(presenter_ids, user.UserId)
			document_ids = append(document_ids, document.DocumentId)
		}

		fmt.Printf("Log: join成功: %s, %d in joinMeeting\n", userId, meetingId)
		return true, meeting.MeetingName, meeting.MeetingStartTime, presenter_names, presenter_ids, document_ids

	} else {
		fmt.Printf("Error: ユーザーもしくは会議が非存在: %s, %d in joinMeeting\n", userId, meetingId)
		return false, "false", time.Now(), []string{}, []string{}, []int{}
	}
}

func exitMeeting(db *gorm.DB, userId string, meetingId int, documentId int) bool {
	var participant Participant
	var question Question
	if participant_err := db.Model(&participant).Where("meeting_id = ? AND user_id = ?", meetingId, userId).Update("is_joining", false).Error; participant_err != nil {
		fmt.Printf("Error: update失敗(参加者の参加状態の更新に失敗しました): %d, %s in exitMeeting\n", meetingId, userId)
		return false
	}
	fmt.Printf("Log: update成功(参加者の参加状態の更新に成功しました): %d, %s in exitMeeting\n", meetingId, userId)
	if delete_question_err := db.First(&question, "user_id = ? AND document_id = ? AND question_ok = ? AND is_voice = ?", userId, documentId, false, true).Delete(&question, "user_id = ? AND document_id = ? AND question_ok = ? AND is_voice = ?", userId, documentId, false, true).Error; delete_question_err != nil {
		fmt.Printf("Log: delete失敗(質問が存在しないか，削除に失敗しました): %s, %d, %t, %t in exitMeeting\n", userId, documentId, false, true)
	} else {
		fmt.Printf("Log: delete成功(質問の削除に成功しました): %s, %d, %t, %t in exitMeeting\n", userId, documentId, false, true)
	}

	fmt.Printf("Log: exit成功: %s, %d in exitMeeting\n", userId, meetingId)
	return true
}

func documentRegister(db *gorm.DB, documentId int, documentUrl string, script string) (bool, int) {
	var document Document
	if err := db.First(&document, "document_id = ?", documentId).Error; err != nil {
		fmt.Printf("Error: 資料が非存在: %d in documentRegister\n", documentId)
		return false, -1
	}
	if documentUrl != "" {
		if document_err := db.Model(&document).Where("document_id = ?", document.DocumentId).Update("document_url", documentUrl).Error; document_err != nil {
			fmt.Printf("Error: update失敗(資料URLの登録に失敗しました): %d in documentRegister\n", document.DocumentId)
			return false, -1
		} else {
			fmt.Printf("Log: update成功(資料URLの登録に成功しました): %d in documentRegister\n", document.DocumentId)
		}
	}
	if script != "" {
		if script_err := db.Model(&document).Where("document_id = ?", document.DocumentId).Update("script", script).Error; script_err != nil {
			fmt.Printf("Error: update失敗(原稿の登録に失敗しました): %d in documentRegister\n", document.DocumentId)
			return false, -1
		} else {
			fmt.Printf("Log: update成功(原稿の登録に成功しました): %d in documentRegister\n", document.DocumentId)
		}
	}

	return true, document.MeetingId
}

func createQuestion(db *gorm.DB, question Question) (bool, int) {
	if err := db.Create(&question).Error; err != nil {
		fmt.Printf("Error: create失敗(質問の登録に失敗しました): %s, %d, %s in createQuestion\n", question.UserId, question.DocumentId, question.QuestionTime)
		return false, -1
	}
	fmt.Printf("Log: create成功(質問の登録に成功しました): %s, %d, %s in createQuestion\n", question.UserId, question.DocumentId, question.QuestionTime)
	return true, question.QuestionId
}

func selectQuestion(db *gorm.DB, meetingId, documentId int, presenterId string, questionUserId string) (bool, bool, string, int) {
	pickQuestioner := true
	suggestQuestion := false
	var question Question
	var participant Participant
	nextQuestionUserId := ""
	location, _ := time.LoadLocation("Asia/Tokyo")

	if voice_question_err := db.First(&question, "document_id = ? AND question_ok = ? AND is_voice = ?", documentId, false, true).Error; voice_question_err == nil {
		if question_err := db.Model(&question).Where("question_id = ?", question.QuestionId).Update("question_ok", true).Error; question_err != nil {
			fmt.Printf("Error: update失敗(質問の回答状況の更新に失敗しました): %d in selectQuestion\n", question.QuestionId)
			return false, false, "", -1
		}
		if incSpeakNum_err := db.Model(&participant).Where("meeting_id = ? AND user_id = ?", meetingId, question.UserId).Update("speak_num", participant.SpeakNum+1).Error; incSpeakNum_err != nil {
			fmt.Printf("Error: update失敗(参加者の話数の更新に失敗しました): %s, %d, %d in selectQuestion\n", participant.UserId, participant.MeetingId, participant.SpeakNum)
			return false, false, "", -1
		}
		nextQuestionUserId = question.UserId
		return pickQuestioner, suggestQuestion, nextQuestionUserId, question.QuestionId
	} else {
		questions := make([]Question, 0, 10)
		if db.Find(&questions, "document_id = ? AND question_ok = ? AND is_voice = ?", documentId, false, false); len(questions) != 0 {
			sort.Sort(ReverseByVoteNum(questions))
			question = questions[0]
			if question_err := db.Model(&question).Where("question_id = ?", question.QuestionId).Update("question_ok", true).Error; question_err != nil {
				fmt.Printf("Error: update失敗(質問の回答状況の更新に失敗しました): %d in selectQuestion\n", question.QuestionId)
				return false, false, "", -1
			}
			pickQuestioner = false
			if incSpeakNum_err := db.Model(&participant).Where("meeting_id = ? AND user_id = ?", meetingId, question.UserId).Update("speak_num", participant.SpeakNum+1).Error; incSpeakNum_err != nil {
				fmt.Printf("Error: update失敗(参加者の話数の更新に失敗しました): %s, %d, %d in selectQuestion\n", participant.UserId, participant.MeetingId, participant.SpeakNum)
				return false, false, "", -1
			}
		}
	}
	if pickQuestioner {
		participants := make([]Participant, 0, 10)
		if db.Find(&participants, "meeting_id = ? AND user_id != ? AND user_id != ? AND is_joining = ?", meetingId, presenterId, questionUserId, true); len(participants) != 0 {
			reactions := make([]Reaction, 0, 10)
			if db.Find(&reactions, "document_id = ? AND suggestion_ok = ?", documentId, false); len(reactions) != 0 {
				sort.Sort(ReverseByReactionNum(reactions))
				if reactions[0].ReactionNum >= len(participants)/2 {
					if reaction_err := db.Model(&reactions[0]).Where("document_id = ? AND document_page = ?", reactions[0].DocumentId, reactions[0].DocumentPage).Update("suggestion_ok", true).Error; reaction_err != nil {
						fmt.Printf("Error: update失敗(資料リアクションの提案状況の更新に失敗しました): %d, %d in selectQuestion\n", reactions[0].DocumentId, reactions[0].DocumentPage)
						return false, false, "", -1
					}
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
					if err := db.Create(&question).Error; err != nil {
						fmt.Printf("Error: create失敗(質問の登録に失敗しました): %s, %d, %s in selectQuestion\n", question.UserId, question.DocumentId, question.QuestionTime)
						return false, false, "", -1
					}
					fmt.Printf("Log: create成功(質問の登録に成功しました): %s, %d, %s in selectQuestion\n", question.UserId, question.DocumentId, question.QuestionTime)
					pickQuestioner = false
					suggestQuestion = true
					return pickQuestioner, suggestQuestion, nextQuestionUserId, question.QuestionId
				}
			}
			sort.Sort(BySpeakNum(participants))
			// rand_max := 3
			// if len(participants) < 3 {
			//	rand_max = len(participants)
			// }
			// participant = participants[rand.Intn(rand_max)]
			participant = participants[0]
			nextQuestionUserId = participant.UserId
			question = Question{
				UserId:       nextQuestionUserId,
				QuestionBody: "",
				DocumentId:   documentId,
				DocumentPage: 1,
				VoteNum:      0,
				QuestionTime: time.Now().In(location),
				QuestionOk:   true,
				IsVoice:      true,
			}
			if err := db.Create(&question).Error; err != nil {
				fmt.Printf("Error: create失敗(質問の登録に失敗しました): %s, %d, %s in selectQuestion\n", question.UserId, question.DocumentId, question.QuestionTime)
				return false, false, "", -1
			}
			if err := db.Model(&participant).Where("user_id = ? AND meeting_id = ?", participant.UserId, participant.MeetingId).Update("speak_num", participant.SpeakNum+1).Error; err != nil {
				fmt.Printf("Error: update失敗(参加者の話数の更新に失敗しました): %s, %d, %d in selectQuestion\n", participant.UserId, participant.MeetingId, participant.SpeakNum)
				return false, false, "", -1
			}
			fmt.Printf("Log: create成功(質問の登録に成功しました): %s, %d, %s in selectQuestion\n", question.UserId, question.DocumentId, question.QuestionTime)

		} else {
			fmt.Printf("Error: 参加者が非存在: %d in selectQuestion\n", meetingId)
			return false, false, "", -1
		}
	}
	return pickQuestioner, suggestQuestion, nextQuestionUserId, question.QuestionId
}

func voteQuestion(db *gorm.DB, questionId int, isVote bool) (int, int, int) {
	var question Question
	var document Document
	if err := db.First(&question, "question_id = ?", questionId).Error; err != nil {
		fmt.Printf("Error: 質問が非存在: %d in voteQuestion\n", questionId)
		return -1, -1, -1
	}
	voteNum := question.VoteNum
	if isVote {
		voteNum += 1
	} else {
		voteNum -= 1
	}
	if err := db.Model(&question).Where("question_id = ?", questionId).Update("vote_num", voteNum).Error; err != nil {
		fmt.Printf("Error: update失敗(質問の投票数の更新に失敗しました): %d, %d in voteQuestion\n", questionId, voteNum)
		return -1, -1, -1
	}

	if err := db.First(&document, "document_id = ?", question.DocumentId).Error; err != nil {
		fmt.Printf("Error: 資料が非存在: %d in voteQuestion\n", question.DocumentId)
		return -1, -1, -1
	}

	return document.MeetingId, questionId, voteNum
}

func handsUp(db *gorm.DB, userId string, documentId int, documentPage int) int {
	var document Document
	location, _ := time.LoadLocation("Asia/Tokyo")

	if document_err := db.First(&document, "document_id = ?", documentId).Error; document_err != nil {
		fmt.Printf("Error: 資料が非存在: %d in handsUp\n", documentId)
		return -1
	}

	if user_err := db.First(&User{}, "user_id = ?", userId).Error; user_err != nil {
		fmt.Printf("Error: ユーザーが非存在: %s in handsUp\n", userId)
		return -1
	}

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
	if question_err := db.Create(&question).Error; question_err != nil {
		fmt.Printf("Error: create失敗(質問の登録に失敗しました): %s, %d, %d, %s in handsUp\n", question.UserId, question.DocumentId, question.DocumentPage, question.QuestionTime)
		return -1
	}
	fmt.Printf("Log: create成功(質問の登録に成功しました): %s, %d, %d, %s in handsUp\n", question.UserId, question.DocumentId, question.DocumentPage, question.QuestionTime)
	return document.MeetingId
}

func handsDown(db *gorm.DB, userId string, documentId int, documentPage int) int {
	var (
		document Document
		question Question
	)

	if document_err := db.First(&document, "document_id = ?", documentId).Error; document_err != nil {
		fmt.Printf("Error: 資料が非存在: %d in handsDown\n", documentId)
		return -1
	}
	if user_err := db.First(&User{}, "user_id = ?", userId).Error; user_err != nil {
		fmt.Printf("Error: ユーザーが非存在: %s in handsDown\n", userId)
		return -1
	}
	if question_err := db.First(&question, "user_id = ? AND document_id = ? AND document_page = ? AND question_ok = ? AND is_voice = ?", userId, document.DocumentId, documentPage, false, true).Error; question_err != nil {
		fmt.Printf("Error: 質問が非存在: %s, %d, %d in handsDown\n", userId, document.DocumentId, documentPage)
		return -1
	}
	if delete_question_err := db.Where("question_id = ?", question.QuestionId).Delete(&question).Error; delete_question_err != nil {
		fmt.Printf("Error: delete失敗(質問の削除に失敗しました): %d in handsDown\n", question.QuestionId)
		return -1
	}
	fmt.Printf("Log: delete成功(質問の削除に成功しました): %d in handsDown\n", question.QuestionId)
	return document.MeetingId
}

func voteReaction(db *gorm.DB, documentId int, documentPage int, isReaction bool) (int, int) {
	var document Document
	var reaction Reaction

	if document_err := db.First(&document, "document_id = ?", documentId).Error; document_err != nil {
		fmt.Printf("Error: 資料が非存在: %d in voteReaction\n", documentId)
		return -1, -1
	}

	if reaction_err := db.First(&reaction, "document_id = ? AND document_page = ?", documentId, documentPage).Error; reaction_err != nil {
		if !isReaction {
			fmt.Printf("Error: 資料リアクションが非存在: %d, %d in voteReaction\n", documentId, documentPage)
			return -1, -1
		}
		reaction = Reaction{
			DocumentId:   document.DocumentId,
			DocumentPage: documentPage,
			ReactionNum:  1,
			SuggestionOk: false,
		}
		if create_reaction_err := db.Create(&reaction).Error; create_reaction_err != nil {
			fmt.Printf("Error: create失敗(資料リアクションの登録に失敗しました): %d, %d in voteReaction\n", reaction.DocumentId, reaction.DocumentPage)
			return -1, -1
		}
		fmt.Printf("Log: create成功(資料リアクションの登録に成功しました): %d, %d in voteReaction\n", reaction.DocumentId, reaction.DocumentPage)
	} else {
		reactionNum := reaction.ReactionNum
		if isReaction {
			reactionNum += 1
		} else {
			reactionNum -= 1
		}
		if update_reaction_num_err := db.Model(&reaction).Where("document_id = ? AND document_page = ?", reaction.DocumentId, reaction.DocumentPage).Update("reaction_num", reactionNum).Error; update_reaction_num_err != nil {
			fmt.Printf("Error: update失敗(資料リアクションのリアクション数の更新に失敗しました): %d, %d in voteReaction\n", reaction.DocumentId, reaction.DocumentPage)
			return -1, -1
		}
		fmt.Printf("Log: update成功(資料リアクションのリアクション数の更新に成功しました): %d, %d in voteReaction\n", reaction.DocumentId, reaction.DocumentPage)
	}
	return document.MeetingId, reaction.ReactionNum
}

func getNextPresenterId(db *gorm.DB, meetingId int, nowPresenterId string) (bool, string, int) {
	var participant Participant
	if participant_err := db.First(&participant, "meeting_id = ? AND user_id = ?", meetingId, nowPresenterId).Error; participant_err != nil {
		fmt.Printf("Error: 参加者が非存在: %s in getNextPresenterId\n", nowPresenterId)
		return false, "", -1
	}
	nextOrder := participant.ParticipantOrder + 1
	if meeting_end_err := db.First(&participant, "meeting_id = ? AND participant_order = ?", meetingId, nextOrder).Error; meeting_end_err != nil {
		fmt.Printf("Log: 会議終了につき次の発表者が非存在: %d in getNextPresenterId\n", nextOrder)
		return true, "", -1
	}
	return false, participant.UserId, nextOrder
}

func getUserName(db *gorm.DB, userId string) string {
	var user User
	if err := db.First(&user, "user_id = ?", userId).Error; err != nil {
		fmt.Printf("Error: ユーザーが非存在: %s in getUserName\n", userId)
		return ""
	}
	return user.UserName
}

func getFirstPresenUserName(db *gorm.DB, meetingId int) string {
	participants := make([]Participant, 0, 10)
	if db.Find(&participants, "meeting_id = ? AND participant_order != -1", meetingId); len(participants) == 0 {
		fmt.Printf("Error: 会議非存在: %d in getFirstPresenUserName\n", meetingId)
		return ""
	}

	sort.Sort(ByParticipantOrder(participants))

	userName := getUserName(db, participants[0].UserId)

	return userName
}

func getQuestionBody(db *gorm.DB, questionId int) (string, int) {
	var question Question
	if err := db.First(&question, "question_id = ?", questionId).Error; err != nil {
		fmt.Printf("Error: 質問が非存在: %d in getQuestionBody\n", questionId)
		return "", -1
	}
	return question.QuestionBody, question.DocumentPage
}

func getQuestionDocumentPage(db *gorm.DB, questionId int) int {
	var question Question
	if err := db.First(&question, "question_id = ?", questionId).Error; err != nil {
		fmt.Printf("Error: 質問が非存在: %d in getQuestionDocumentPage\n", questionId)
		return -1
	}
	return question.DocumentPage
}

func getDocumentId(db *gorm.DB, userId string, meetingId int) int {
	var document Document
	if err := db.First(&document, "user_id = ? AND meeting_id = ?", userId, meetingId).Error; err != nil {
		fmt.Printf("Error: 資料が非存在: %s, %d in getDocumentId\n", userId, meetingId)
		return -1
	}
	return document.DocumentId
}

func documentGet(db *gorm.DB, documentId int) (bool, string, string) {
	var (
		document    Document
		documentUrl *string
		script      *string
		emptyString = ""
	)

	if err := db.First(&document, "document_id = ?", documentId).Error; err != nil {
		fmt.Printf("Error: 資料が非存在: %d in documentGet\n", documentId)
		return false, "", ""
	}
	if documentUrl = document.DocumentUrl; documentUrl == nil {
		fmt.Printf("Log: 資料URLが非存在: %d in documentGet\n", documentId)
		documentUrl = &emptyString
	}
	if script = document.Script; script == nil {
		fmt.Printf("Log: 原稿が非存在: %d in documentGet\n", documentId)
		script = &emptyString
	}
	return true, *documentUrl, *script
}

func questionsGet(db *gorm.DB, meetingId int) (bool, int, []int, []string, []int, []int, []string, []string, []int) {
	var (
		layout        = "2006/01/02 15:04:05"
		location, _   = time.LoadLocation("Asia/Tokyo")
		questions     = make([]QuestionAndPresenterId, 0, 10)
		questionIds   = make([]int, 0, 10)
		questionBodys = make([]string, 0, 10)
		documentIds   = make([]int, 0, 10)
		documentPages = make([]int, 0, 10)
		questionTimes = make([]string, 0, 10)
		presenterIds  = make([]string, 0, 10)
		voteNums      = make([]int, 0, 10)
	)
	if db.Table("documents").Select("questions.question_id, questions.question_body, questions.document_id, questions.document_page, questions.question_time, documents.user_id, questions.vote_num").Where("documents.meeting_id = ?", meetingId).Joins("right join questions on documents.document_id = questions.document_id").Scan(&questions); len(questions) == 0 {
		fmt.Printf("Log: 質問が非存在: %d in questionsGet\n", meetingId)
		return false, meetingId, []int{}, []string{}, []int{}, []int{}, []string{}, []string{}, []int{}
	}
	for _, q := range questions {
		questionIds = append(questionIds, q.QuestionId)
		questionBodys = append(questionBodys, q.QuestionBody)
		documentIds = append(documentIds, q.DocumentId)
		documentPages = append(documentPages, q.DocumentPage)
		questionTimes = append(questionTimes, q.QuestionTime.In(location).Format(layout))
		presenterIds = append(presenterIds, q.UserId)
		voteNums = append(voteNums, q.VoteNum)
	}

	return true, meetingId, questionIds, questionBodys, documentIds, documentPages, questionTimes, presenterIds, voteNums
}

func getPresenterId(db *gorm.DB, documentId int) string {
	var document Document
	if err := db.First(&document, "document_id = ?", documentId).Error; err != nil {
		fmt.Printf("Error: 資料が非存在: %d in getPresenterId\n", documentId)
		return ""
	}
	return document.UserId
}

func setMeetingDone(db *gorm.DB, meetingId int) {
	var meeting Meeting
	db.First(&meeting, "meeting_id = ?", meetingId)
	db.Model(&meeting).Where("meeting_id = ?", meetingId).Update("meeting_done", true)
}
