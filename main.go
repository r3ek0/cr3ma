package main

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/r3ek0/o3"
)

type JsonMessage struct {
	To  string `json:"to"`
	Msg string `json:"msg"`
}

func checkSaveContact(ctx *o3.SessionContext, tr *o3.ThreemaRest, contact string, abpath string) {
	if _, b := ctx.ID.Contacts.Get(contact); b == false {
		log.Printf("Looking up %s from directory server\n", contact)
		// retrieve the ID from Threema's servers
		receipient := o3.NewIDString(contact)
		log.Printf("Retrieving %s from directory server\n", receipient.String())
		newContact, err := tr.GetContactByID(receipient)
		if err != nil {
			log.Fatal(err)
		}
		// add to address book
		log.Printf("Adding contact: %s - %s\n", newContact.ID, newContact.Name)
		ctx.ID.Contacts.Add(newContact)

		// save address book
		log.Printf("Saving addressbook to %s\n", abpath)
		err = ctx.ID.Contacts.SaveTo(abpath)
		if err != nil {
			log.Println("saving addressbook failed")
			log.Fatal(err)
		}
	}

}

func forwardMessage(ctx *o3.SessionContext, smc chan<- o3.Message, tr *o3.ThreemaRest, msg JsonMessage) {
	// send the message
	err := ctx.SendTextMessage(msg.To, msg.Msg, smc)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("[%-20s] %-20s -> %s", msg.To, ctx.ID.Nick.String(), msg.Msg)
}

func forwardGroupMessage(ctx *o3.SessionContext, smc chan<- o3.Message, tr *o3.ThreemaRest, msg JsonMessage) {
	// send the message
	groupid, _ := ctx.ID.Groups.ReadGroupID(msg.To)

	idx, group := ctx.ID.Groups.GetGroupById(groupid)
	if idx < 0 {
		log.Printf("ERROR sending to group [%s].\n", msg.To)
	} else {
		time.Sleep(500 * time.Millisecond)
		nick := ctx.ID.Nick.String()
		ctx.SendGroupTextMessage(group, msg.Msg, smc)
		log.Printf("[%-20s] [%-20s] -> %s", group.Name, nick, msg.Msg)
	}

}

func main() {

	var (
		envpass            = os.Getenv("CR3MAPASS")
		cr3mahome          = os.Getenv("CR3MAHOME")
		pass               = []byte(envpass)
		tr                 o3.ThreemaRest
		idpath             = "threema.id"
		abpath             = "address.book"
		gdpath             = "group.directory"
		tid                o3.ThreemaID
		pubnick            = os.Getenv("CR3MANICK")
		jsonReceiverSocket = os.Getenv("CR3MASOCKET")
	)

	if len(jsonReceiverSocket) < 1 {
		log.Println("Environment varibale not set: CR3MASOCKET")
		log.Println("example: CR3MASOCKET=localhost:8082")
		os.Exit(1)
	}

	if len(envpass) < 1 {
		log.Println("Environment varibale not set: CR3MAPASS")
		log.Println("example: CR3MAPASS=test123")
		os.Exit(1)
	}

	if len(cr3mahome) > 0 {
		idpath = cr3mahome + "/" + idpath
		abpath = cr3mahome + "/" + abpath
		gdpath = cr3mahome + "/" + gdpath
	}

	if len(pubnick) < 1 {
		pubnick = "cr3ma-test-bot"
	}

	// check whether an id file exists or else create a new one
	if _, err := os.Stat(idpath); err != nil {
		var err error
		tid, err = tr.CreateIdentity()
		if err != nil {
			log.Println("CreateIdentity failed")
			log.Fatal(err)
		}
		log.Printf("Saving ID to %s\n", idpath)
		err = tid.SaveToFile(idpath, pass)
		if err != nil {
			log.Println("saving ID failed")
			log.Fatal(err)
		}
	} else {
		log.Printf("Loading ID from %s\n", idpath)
		tid, err = o3.LoadIDFromFile(idpath, pass)
		if err != nil {
			log.Fatal(err)
		}
	}
	log.Printf("Using ID: %s\n", tid.String())

	tid.Nick = o3.NewPubNick(pubnick)

	ctx := o3.NewSessionContext(tid)

	//check if we can load an addressbook
	if _, err := os.Stat(abpath); !os.IsNotExist(err) {
		log.Printf("Loading addressbook from %s\n", abpath)
		err = ctx.ID.Contacts.LoadFromFile(abpath)
		if err != nil {
			log.Println("loading addressbook failed")
			log.Fatal(err)
		}
	}

	//check if we can load a group directory
	if _, err := os.Stat(gdpath); !os.IsNotExist(err) {
		log.Printf("Loading group directory from %s\n", gdpath)
		err = ctx.ID.Groups.LoadFromFile(gdpath)
		if err != nil {
			log.Println("loading group directory failed")
			log.Fatal(err)
		}
	}

	// let the session begin
	log.Println("Starting session")
	sendMsgChan, receiveMsgChan, err := ctx.Run()
	if err != nil {
		log.Fatal(err)
	}

	http.HandleFunc("/send", func(rw http.ResponseWriter, req *http.Request) {
		body, err := ioutil.ReadAll(req.Body)
		if err != nil {
			panic(err)

		}

		var msg JsonMessage
		err = json.Unmarshal(body, &msg)
		if err != nil {
			panic(err)
		}
		defer req.Body.Close()

		checkSaveContact(&ctx, &tr, msg.To, abpath)
		forwardMessage(&ctx, sendMsgChan, &tr, msg)
	})

	http.HandleFunc("/sendGroup", func(rw http.ResponseWriter, req *http.Request) {
		body, err := ioutil.ReadAll(req.Body)
		if err != nil {
			panic(err)

		}

		var msg JsonMessage
		err = json.Unmarshal(body, &msg)
		if err != nil {
			panic(err)
		}
		defer req.Body.Close()

		forwardGroupMessage(&ctx, sendMsgChan, &tr, msg)
	})

	go http.ListenAndServe(jsonReceiverSocket, nil)
	log.Printf("Json receiver on port %s", jsonReceiverSocket)

	// handle incoming messages
	for receivedMessage := range receiveMsgChan {
		if receivedMessage.Err != nil {
			log.Printf("Error Receiving Message: %s\n", receivedMessage.Err)
			continue
		}
		switch msg := receivedMessage.Msg.(type) {
		case o3.ImageMessage:
			// display the image if you like
		case o3.AudioMessage:
			// play the audio if you like
		case o3.TextMessage:
			// but only if it's no a message we sent to ourselves, avoid recursive neverending qoutes
			if tid.String() == msg.Sender().String() {
				continue
			}
			log.Printf("[%-20s] -> %s", msg.PubNick(), msg.Text())

			// confirm to the sender that we received the message
			// this is how one can send messages manually without helper functions like "SendTextMessage"
			drm, err := o3.NewDeliveryReceiptMessage(&ctx, msg.Sender().String(), msg.ID(), o3.MSGDELIVERED)
			if err != nil {
				log.Fatal(err)
			}
			sendMsgChan <- drm

			// show message read to rid
			upm, err := o3.NewDeliveryReceiptMessage(&ctx, msg.Sender().String(), msg.ID(), o3.MSGREAD)
			if err != nil {
				log.Fatal(err)
			}
			sendMsgChan <- upm

		case o3.GroupTextMessage:
			idx, group := ctx.ID.Groups.GetGroupById(msg.GroupID())
			if idx < 0 {
				log.Fatal("no such group")
			}
			log.Printf("[%-20s] %-20s -> %s", group.Name, msg.PubNick(), msg.Text())
		case o3.GroupManageSetNameMessage:
			// We need to update the group dir
			idx, group := ctx.ID.Groups.GetGroupById(msg.GroupID())
			if idx < 0 {
				var group o3.Group
				group.GroupID = msg.GroupID()
				group.CreatorID = msg.Sender()
				group.Name = msg.Name()
				group.Members = append(group.Members, msg.Sender())
				group.Members = append(group.Members, ctx.ID.ID)

				ctx.ID.Groups.AddGroup(group)
			}
			group.Name = msg.Name()
			ctx.ID.Groups.UpdateGroup(group)
			ctx.ID.Groups.SaveToFile(gdpath)

			log.Printf("Group [%x] is now called %s\n", msg.GroupID(), msg.Name())

		case o3.GroupManageSetMembersMessage:
			log.Printf("Group [%x] member update\n", msg.GroupID())
			idx, group := ctx.ID.Groups.GetGroupById(msg.GroupID())
			members := msg.Members()
			if idx > 0 {
				// replace our id with group creator id
				// \bc we know we are in the group, but we don't know who the creator is
				for i := range members {
					if members[i] == ctx.ID.ID {
						members[i] = msg.Sender()
					}
				}
			}

			group.Members = members
			// TODO: add only adds if the group is new so updates on the member list do not work yet
			//ctx.ID.Groups.Upsert(o3.Group{CreatorID: msg.Sender(), GroupID: msg.GroupID(), Members: members})
			ctx.ID.Groups.UpdateGroup(group)
			ctx.ID.Groups.SaveToFile(gdpath)
			log.Printf("Group [%x] now includes %v\n", msg.GroupID(), msg.Members())

		case o3.GroupMemberLeftMessage:
			//log.Printf("Member [%s] left the Group [%x]\n", msg.PubNick(), msg.GroupID())
			continue
		case o3.DeliveryReceiptMessage:
			//log.Printf("Message [%x] has been acknowledged by the server. %s\n", msg.MsgID(), msg.GetPrintableContent())
			continue
		case o3.TypingNotificationMessage:
			//log.Printf("Typing Notification from %s: [%x]\n", msg.PubNick(), msg.OnOff)
			continue
		default:
			log.Printf("Unknown message type from: %s\nContent: %#v", msg.Sender(), msg)
		}
	}

}
