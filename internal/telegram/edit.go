package telegram

import (
	"fmt"
	"strings"
	"time"

	cmap "github.com/orcaman/concurrent-map"
	log "github.com/sirupsen/logrus"
	tb "gopkg.in/lightningtipbot/telebot.v2"
)

var editStack cmap.ConcurrentMap

type edit struct {
	to       tb.Editable
	what     interface{}
	options  []interface{}
	lastEdit time.Time
	edited   bool
}

func init() {
	editStack = cmap.New()
}

const resultTrueError = "telebot: result is True"
const editSameStringError = "specified new message content and reply markup are exactly the same as a current content and reply markup of the message"

// startEditWorker will loop through the editStack and run tryEditMessage on not edited messages.
// if editFromStack is older than 5 seconds, editFromStack will be removed.
func (bot TipBot) startEditWorker() {
	go func() {
		for {
			for _, k := range editStack.Keys() {
				if e, ok := editStack.Get(k); ok {
					editFromStack := e.(edit)
					if !editFromStack.edited {
						_, err := bot.tryEditMessage(editFromStack.to, editFromStack.what, editFromStack.options...)
						if err != nil && err.Error() != resultTrueError && !strings.Contains(err.Error(), editSameStringError) {
							// any other error should not be ignored
							log.Tracef("[startEditWorker] Skip edit error: %s", err.Error())

						} else {
							if err != nil {
								log.Tracef("[startEditWorker] Edit error: %s", err.Error())
							}
							log.Tracef("[startEditWorker] message from stack edited %+v", editFromStack)
							editFromStack.lastEdit = time.Now()
							editFromStack.edited = true
							editStack.Set(k, editFromStack)
						}
					} else {
						if editFromStack.lastEdit.Before(time.Now().Add(-(time.Duration(5) * time.Second))) {
							log.Tracef("[startEditWorker] removing message edit from stack %+v", editFromStack)
							editStack.Remove(k)
						}
					}
				}
			}
			time.Sleep(time.Millisecond * 500)
		}
	}()

}

// tryEditStack will add the editable to the edit stack, if what (message) changed.
func (bot TipBot) tryEditStack(to tb.Editable, what interface{}, options ...interface{}) {
	msgSig, chat := to.MessageSig()
	var sig = fmt.Sprintf("%s-%d", msgSig, chat)
	if e, ok := editStack.Get(sig); ok {
		editFromStack := e.(edit)
		if editFromStack.what == what.(string) {
			log.Tracef("[tryEditStack] Message already in edit stack. Skipping")
			return
		}
	}
	e := edit{options: options, what: what, to: to}

	editStack.Set(sig, e)
	log.Tracef("[tryEditStack] Added message %s to edit stack. len(editStack)=%d", sig, len(editStack.Keys()))
}
