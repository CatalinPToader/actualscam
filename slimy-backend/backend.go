package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"github.com/emicklei/go-restful/v3"
	"io"
	"log"
	"net/http"
	"path"
	"time"
)

type FightStatus uint8
type WildSlimeStatus uint8
type Move uint8

const (
	WaitingOnPlayer FightStatus = iota
	WaitingOnTx
	PlayerLost
	PlayerWon
)

const (
	Fighting WildSlimeStatus = iota
	CanCatch
)

const (
	Attack Move = iota
	Buff
	Heal
	Catch
)

type SlimeFight struct {
	Attack uint8
	Def    uint8
	MaxHP  uint8
	CurrHP uint8
}

type FightMove struct {
	Hunter     string `json:"hunter"`
	MovePicked Move   `json:"move"`
}

type FightInit struct {
	Hunter  string `json:"hunter"`
	SlimeID uint64 `json:"slime_id"`
}

type Fight struct {
	Wild       SlimeFight      `json:"wild"`
	Attacker   SlimeFight      `json:"attacker"`
	Hunter     string          `json:"hunter"`
	Status     FightStatus     `json:"status"`
	WildStatus WildSlimeStatus `json:"wild_status"`
}

type Slime struct {
	SlimeID uint64 `json:"slime_id"`
}

type Database struct {
	Fights map[uint]Fight
	Users  map[string]uint64
}

func main() {
	ws := new(restful.WebService)

	ws.Route(ws.GET("/register_user/{tx}").
		Produces(restful.MIME_JSON).
		Doc("Registers new user").
		To(handleRegister))

	ws.Route(ws.GET("/item/{tx}").
		Produces(restful.MIME_JSON).
		Doc("Acknowledges item use.").
		To(handleItem))

	ws.Route(ws.GET("/catch/{tx}").
		Produces(restful.MIME_JSON).
		Doc("Acknowledges catch").
		To(handleCatch))

	ws.Route(ws.POST("/breed/{tx}").
		Consumes(restful.MIME_JSON).
		Doc("Breeds slimes").
		Returns(200, "OK", Slime{}).
		Returns(500, "Internal Server Error", nil).
		To(handleBreed))

	ws.Route(ws.GET("/gen/{tx}").
		Produces(restful.MIME_JSON).
		Doc("Generates wild slimes").
		Returns(200, "OK", Slime{}).
		Returns(500, "Internal Server Error", nil).
		To(handleGen))

	ws.Route(ws.GET("/fight/{wild_slime_id}").
		Produces(restful.MIME_JSON).
		Doc("Starts a fight").
		Reads(FightInit{}).
		Writes(Fight{}). // on the response
		Returns(200, "OK", Fight{}).
		Returns(500, "Internal Server Error", nil).
		To(handleFightStart))

	ws.Route(ws.POST("/fight_move/{wild_slime_id}").
		Produces(restful.MIME_JSON).
		Reads(FightMove{}).
		Writes(Fight{}).
		Doc("Sends a fight move").
		Returns(200, "OK", Fight{}).
		Returns(500, "Internal Server Error", nil).
		To(handleFightMoves))

	cors := restful.CrossOriginResourceSharing{
		AllowedHeaders: []string{"Content-Type", "Accept"},
		AllowedDomains: []string{},
		AllowedMethods: []string{"POST"},
		CookiesAllowed: true,
		Container:      restful.DefaultContainer}
	restful.DefaultContainer.Filter(cors.Filter)
	restful.Add(ws)

	log.Fatal(http.ListenAndServe(":8080", nil))
}

func handleSubFile(req *restful.Request, resp *restful.Response) {
	actual := path.Join(req.PathParameter("subpath"))
	fmt.Printf("serving %s ... (from %s)\n", actual, req.PathParameter("subpath"))
	http.ServeFile(
		resp.ResponseWriter,
		req.Request,
		actual)
}

func handleFile(req *restful.Request, resp *restful.Response) {
	http.ServeFile(
		resp.ResponseWriter,
		req.Request,
		path.Join(req.QueryParameter("resource")))
}

func getCookie(req *restful.Request, cookieName string) *http.Cookie {
	if req.Request == nil || req.Request.Cookies() == nil {
		return nil
	}
	cookies := req.Request.Cookies()
	for _, cookie := range cookies {
		if cookie.Name == cookieName {
			return cookie
		}
	}

	return nil
}

func updateUserStatus(user User, online bool) error {
	if online {
		_, err := db.Exec("UPDATE users SET online = $1 where cookie = $2", online, user.cookieVal)
		return err
	} else {
		_, err := db.Exec("UPDATE users SET online = $1, lastchannel=NULL where cookie = $2", online, user.cookieVal)
		return err
	}
}

func handleHeartbeatz(req *restful.Request, resp *restful.Response) {
	cookie := getCookie(req, "ChatUserAuth")
	if cookie == nil {
		resp.WriteHeader(http.StatusUnauthorized)
		return
	}

	cookieVal := cookie.Value
	user := User{cookieVal: cookieVal}
	users.Put(user, time.Now())

	err := updateUserStatus(user, true)
	if err != nil {
		log.Printf("DB error update user status %v", err)
		resp.WriteHeader(http.StatusInternalServerError)
	}

	return
}

func handleChannelList(req *restful.Request, resp *restful.Response) {
	cookie := getCookie(req, "ChatUserAuth")
	if cookie == nil {
		resp.WriteHeader(http.StatusUnauthorized)
		return
	}

	cookieVal := cookie.Value
	user := User{cookieVal: cookieVal}

	userID, err := getUserID(user)
	if err != nil {
		resp.WriteHeader(http.StatusInternalServerError)
		return
	}

	var res *sql.Rows
	res, err = db.Query("SELECT channel_name FROM listChannels WHERE id in (SELECT channelID from allowedChannel where userID = $1) OR public = true", userID)
	if err != nil {
		log.Printf("DB error query channel list %v", err)
		resp.WriteHeader(http.StatusInternalServerError)
		return
	}

	var channel string
	var channels []string

	for res.Next() {
		err = res.Scan(&channel)
		if err != nil {
			log.Printf("DB error scan channel name %v", err)
			resp.WriteHeader(http.StatusInternalServerError)
			return
		}

		channels = append(channels, channel)
	}

	channelList := ChannelList{List: channels}

	err = resp.WriteAsJson(channelList)
	if err != nil {
		log.Printf("Writing channel list to response error %v", err)
		resp.WriteHeader(http.StatusInternalServerError)
		return
	}

	return
}

func handleChannel(req *restful.Request, resp *restful.Response) {
	cookie := getCookie(req, "ChatUserAuth")
	if cookie == nil {
		resp.WriteHeader(http.StatusUnauthorized)
		return
	}

	cookieVal := cookie.Value
	channelName := req.PathParameter("channelName")

	byteArr, err := io.ReadAll(req.Request.Body)
	if err != nil {
		log.Printf("Error on reading req body %v", req)
		resp.WriteHeader(http.StatusInternalServerError)
		return
	}
	var msg Message

	err = json.Unmarshal(byteArr, &msg)
	if err != nil {
		log.Printf("Could not unmarshall message")
		resp.WriteHeader(http.StatusBadRequest)
		return
	}

	user := User{cookieVal: cookieVal}

	userID, err := getUsername(user)
	if err != nil {
		return
	}

	_, err = db.Exec(fmt.Sprintf("INSERT INTO %s (username, msg) VALUES ($1, $2)", channelName), userID, msg.Message)
	if err != nil {
		log.Printf("DB error insert user message %v", err)
		resp.WriteHeader(http.StatusInternalServerError)
		return
	}

	_, err = db.Exec("UPDATE users SET lastChannel = (SELECT id from listChannels where channel_name=$1) WHERE cookie=$2", channelName, cookieVal)
	if err != nil {
		log.Printf("DB error update user channel %v", err)
		resp.WriteHeader(http.StatusInternalServerError)
		return
	}

	return
}

func handleChannelUsers(req *restful.Request, resp *restful.Response) {
	cookie := getCookie(req, "ChatUserAuth")
	if cookie == nil {
		resp.WriteHeader(http.StatusUnauthorized)
		return
	}

	cookieVal := cookie.Value
	channelName := req.PathParameter("channelName")

	user := User{cookieVal: cookieVal}

	_, err := getUserID(user)
	if err != nil {
		resp.WriteHeader(http.StatusInternalServerError)
		return
	}

	res, err := db.Query("SELECT id FROM listChannels where channel_name=$1", channelName)
	if err != nil {
		log.Printf("DB error query channel id %v", err)
		resp.WriteHeader(http.StatusInternalServerError)
		return
	}
	res.Next()
	var channelID string
	err = res.Scan(&channelID)
	if err != nil {
		log.Printf("DB error scan channel id %v", err)
		resp.WriteHeader(http.StatusInternalServerError)
		return
	}

	err = res.Close()
	if err != nil {
		log.Printf("DB error close %v", err)
		resp.WriteHeader(http.StatusInternalServerError)
		return
	}

	res, err = db.Query("SELECT username FROM users WHERE online = true AND lastChannel = $1", channelID)
	if err != nil {
		log.Printf("DB error query online users %v", err)
		resp.WriteHeader(http.StatusInternalServerError)
		return
	}

	var username string
	var usersInChannel []string

	for res.Next() {
		err = res.Scan(&username)
		if err != nil {
			log.Printf("DB error scan username for online users %v", err)
			resp.WriteHeader(http.StatusInternalServerError)
			return
		}

		usersInChannel = append(usersInChannel, username)
	}

	userList := UserList{List: usersInChannel}

	err = resp.WriteAsJson(userList)
	if err != nil {
		log.Printf("Writing user list to response error %v", err)
		resp.WriteHeader(http.StatusInternalServerError)
		return
	}

	return
}

func getUserID(user User) (string, error) {
	res, err := db.Query("SELECT id FROM users where cookie=$1", user.cookieVal)
	if err != nil {
		log.Printf("DB error query user id %v", err)
		return "", err
	}

	res.Next()
	var userID string
	err = res.Scan(&userID)
	if err != nil {
		log.Printf("DB error scan user id %v", err)
		return "", err
	}
	err = res.Close()
	if err != nil {
		log.Printf("DB error close %v", err)
		return "", err
	}
	return userID, nil
}

func getUsername(user User) (string, error) {
	res, err := db.Query("SELECT username FROM users where cookie=$1", user.cookieVal)
	if err != nil {
		log.Printf("DB error query username %v", err)
		return "", err
	}

	res.Next()
	var userID string
	err = res.Scan(&userID)
	if err != nil {
		log.Printf("DB error scan username %v", err)
		return "", err
	}
	err = res.Close()
	if err != nil {
		log.Printf("DB error close %v", err)
		return "", err
	}
	return userID, nil
}

func handleChannelGET(req *restful.Request, resp *restful.Response) {
	channelName := req.PathParameter("channelName")

	res, err := db.Query(fmt.Sprintf("SELECT username, extract(epoch from stamp)::bigint, msg FROM %s LIMIT 100", channelName))
	if err != nil {
		log.Printf("DB error query channel messages %v", err)
		resp.WriteHeader(http.StatusInternalServerError)
		return
	}

	var username string
	var unix int64
	var msg string
	var messages []ChannelMessage

	for res.Next() {
		err = res.Scan(&username, &unix, &msg)
		if err != nil {
			log.Printf("DB error scan user, stamp, message %v", err)
			resp.WriteHeader(http.StatusInternalServerError)
			return
		}

		messages = append(messages, ChannelMessage{Message: msg, Timestamp: unix, User: username})
	}

	messageList := ChannelMessageList{List: messages}

	err = resp.WriteAsJson(messageList)
	if err != nil {
		log.Printf("Writing message list to response error %v", err)
		resp.WriteHeader(http.StatusInternalServerError)
		return
	}

	return
}
