package kiam

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"git.kanosolution.net/kano/appkit"
	"github.com/google/uuid"
	"github.com/sebarcode/codekit"
	"github.com/sebarcode/logger"
)

type Session struct {
	SessionID   string
	ReferenceID string
	Data        codekit.M
	LastUpdate  time.Time
	Duration    int
}

type SessionPool struct {
	mtx      *sync.RWMutex
	sessions map[string]*Session
	refs     map[string]string
	logger   *logger.LogEngine

	CleanUpDuration time.Duration
	CleanUpCheck    time.Duration
}

func NewSessionPool(log *logger.LogEngine) *SessionPool {
	if log == nil {
		log = appkit.Log()
	}
	sp := new(SessionPool)
	sp.mtx = new(sync.RWMutex)
	sp.logger = log
	sp.sessions = map[string]*Session{}
	sp.refs = map[string]string{}

	if int(sp.CleanUpDuration) == 0 {
		sp.CleanUpDuration = 30 * time.Minute
	}

	if int(sp.CleanUpCheck) == 0 {
		sp.CleanUpCheck = 100 * time.Millisecond
	}

	go func() {
		for {
			<-time.After(sp.CleanUpCheck)
			cleaned := sp.RemoveSessionByDuration(sp.CleanUpDuration)
			if cleaned > 0 {
				sp.logger.Infof("cleaning up %d sessions(s)", cleaned)
			}
		}
	}()

	return sp
}

func (sp *SessionPool) GetIDs() []string {
	res := []string{}
	if sp.sessions == nil {
		sp.sessions = map[string]*Session{}
	}
	for k := range sp.sessions {
		res = append(res, k)
	}
	return res
}

func (sp *SessionPool) GetBySessionID(id string) (*Session, bool) {
	sp.mtx.RLock()
	defer sp.mtx.RUnlock()

	se, ok := sp.sessions[id]
	return se, ok
}

func (sp *SessionPool) GetByReferenceID(id string) (*Session, bool) {
	sp.mtx.RLock()
	defer sp.mtx.RUnlock()

	seid, ok := sp.refs[id]
	if seid == "" || !ok {
		return nil, false
	}

	se, ok := sp.sessions[seid]
	return se, ok
}

func (sp *SessionPool) Create(referenceID string, data codekit.M, second int) (*Session, error) {
	_, ok := sp.GetByReferenceID(referenceID)
	if ok {
		return nil, errors.New("Session already exist")
	}

	if second == 0 {
		second = int(sp.CleanUpDuration) / int(time.Second)
	}

	se := new(Session)
	se.SessionID = uuid.New().String()
	se.ReferenceID = referenceID
	se.Data = data
	se.Duration = second
	se.LastUpdate = time.Now()

	sp.mtx.Lock()
	defer sp.mtx.Unlock()
	sp.sessions[se.SessionID] = se
	sp.refs[referenceID] = se.SessionID
	return se, nil
}

func (sp *SessionPool) RegisterSession(sess *Session) error {
	if sess.SessionID == "" || sess.ReferenceID == "" {
		return fmt.Errorf("sessionID and referenceID are mandatory")
	}

	sp.mtx.Lock()
	defer sp.mtx.Unlock()

	sp.sessions[sess.SessionID] = sess
	sp.refs[sess.ReferenceID] = sess.SessionID
	return nil
}

func (sp *SessionPool) Update(sessionID string, second int) error {
	se, ok := sp.GetBySessionID(sessionID)
	if !ok {
		return errors.New("Session for this ID is not exist")
	}

	if second != 0 {
		se.Duration = second
	}
	se.LastUpdate = time.Now()

	sp.mtx.Lock()
	defer sp.mtx.Unlock()
	sp.sessions[se.SessionID] = se
	return nil
}

func (sp *SessionPool) Renew(sessionID string, second int) (string, error) {
	se, ok := sp.GetBySessionID(sessionID)
	if !ok {
		return "", errors.New("Session for this ID is not exist")
	}

	if second != 0 {
		se.Duration = second
	}

	newse := *se
	newse.LastUpdate = time.Now()
	newse.SessionID = uuid.New().String()

	sp.mtx.Lock()
	defer sp.mtx.Unlock()

	// add new session
	sp.sessions[newse.SessionID] = &newse
	sp.refs[newse.ReferenceID] = newse.SessionID

	// remove old one
	delete(sp.sessions, sessionID)

	return newse.SessionID, nil
}

func (sp *SessionPool) Remove(sessionID string) {
	se, ok := sp.GetBySessionID(sessionID)
	if !ok {
		return
	}

	sp.mtx.Lock()
	defer sp.mtx.Unlock()
	delete(sp.sessions, se.SessionID)
	delete(sp.refs, se.ReferenceID)
}

func (sp *SessionPool) RemoveSessionByDuration(d time.Duration) int {
	ss := []*Session{}

	sp.mtx.RLock()
	now := time.Now()
	for _, sess := range sp.sessions {
		if now.After(sess.LastUpdate.Add(time.Duration(sess.Duration) * time.Second)) {
			ss = append(ss, sess)
		}
	}
	sp.mtx.RUnlock()

	sp.mtx.Lock()
	for _, s := range ss {
		delete(sp.sessions, s.SessionID)
		delete(sp.refs, s.ReferenceID)
		s = nil
	}
	sp.mtx.Unlock()

	return len(ss)
}

func (sp *SessionPool) Length() int {
	sp.mtx.RLock()
	defer sp.mtx.Unlock()
	return len(sp.sessions)
}
