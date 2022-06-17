package kiam

import (
	"errors"

	"git.kanosolution.net/kano/kaos"
	"github.com/dgrijalva/jwt-go"
	"github.com/sebarcode/codekit"
	"github.com/sebarcode/logger"
)

type Options struct {
	Storage           IAMStorage
	SignMethod        jwt.SigningMethod
	SignSecret        string
	AllowMultiSession bool
	MultiSession      int
	SecondLifeTime    int
}

type Manager struct {
	pool           *SessionPool
	secondLifeTime int
	opts           Options
}

func NewIAM(logger *logger.LogEngine, secondLifeTime int, opt *Options) *Manager {
	ae := new(Manager)
	ae.pool = NewSessionPool(logger)
	if secondLifeTime == 0 {
		secondLifeTime = 60 * 60 * 24 * 7
	}
	ae.secondLifeTime = secondLifeTime
	if opt == nil {
		ae.opts = Options{}
	} else {
		ae.opts = *opt
	}
	if ae.opts.SecondLifeTime == 0 {
		ae.opts.SecondLifeTime = ae.secondLifeTime
	}
	return ae
}

func (m *Manager) Options() Options {
	return m.opts
}

func (a *Manager) Get(ctx *kaos.Context, parm codekit.M) (*Session, error) {
	var err error
	id := parm.GetString("ID")
	if id == "" {
		return nil, errors.New("ID is mandatory")
	}

	session, ok := a.pool.GetBySessionID(id)
	if !ok {
		if a.opts.Storage == nil {
			return nil, errors.New("Session not found")
		}

		session, err = a.opts.Storage.Get(id)
		if err != nil {
			return nil, errors.New("Session not found. " + err.Error())
		}

		// session found from storage, update session pool
		func() {
			a.pool.mtx.Lock()
			defer a.pool.mtx.Unlock()
			a.pool.sessions[session.SessionID] = session
			a.pool.refs[session.ReferenceID] = session.SessionID
		}()
	}

	a.pool.Update(session.SessionID, 0)
	return session, nil
}

func (a *Manager) Create(ctx *kaos.Context, parm codekit.M) (*Session, error) {
	id := parm.GetString("ID")
	duration := parm.GetInt("Second")
	if duration == 0 {
		duration = a.secondLifeTime
	}

	if id == "" {
		return nil, errors.New("ID is mandatory")
	}

	s, e := a.pool.Create(id, nil, duration)
	if e == nil && a.opts.Storage != nil {
		go a.opts.Storage.Write(s)
	}
	return s, e
}

func (a *Manager) FindOrCreate(ctx *kaos.Context, parm codekit.M, data codekit.M) (*Session, error) {
	id := parm.GetString("ID")
	duration := parm.GetInt("Second")
	if duration == 0 {
		duration = a.secondLifeTime
	}

	if id == "" {
		return nil, errors.New("ID is mandatory")
	}

	s, ok := a.pool.GetByReferenceID(id)
	if !ok {
		s, e := a.pool.Create(id, data, duration)
		if e == nil && a.opts.Storage != nil {
			go a.opts.Storage.Write(s)
		}
		return s, e
	}
	return s, nil
}

func (a *Manager) Renew(ctx *kaos.Context, parm codekit.M) (*Session, error) {
	id := parm.GetString("ID")
	duration := parm.GetInt("Second")
	if id == "" {
		return nil, errors.New("ID is mandatory")
	}

	seid, e := a.pool.Renew(id, duration)
	if e != nil {
		return nil, e
	}
	se, _ := a.pool.GetBySessionID(seid)
	if a.opts.Storage != nil {
		go a.opts.Storage.Write(se)
	}
	return se, nil
}

func (a *Manager) Remove(ctx *kaos.Context, parm codekit.M) (string, error) {
	id := parm.GetString("ID")

	se, _ := a.pool.GetBySessionID(id)
	if se == nil {
		return "", nil
	}

	delete(a.pool.refs, se.ReferenceID)
	delete(a.pool.sessions, se.SessionID)

	if a.opts.Storage != nil {
		go a.opts.Storage.Remove(se.SessionID)
	}
	return "", nil
}

func (a *Manager) Store() error {
	if a.opts.Storage != nil {
		return a.opts.Storage.Store(a.pool)
	}
	return nil
}

func (a *Manager) Load() error {
	if a.opts.Storage != nil {
		return a.opts.Storage.Load(a.pool)
	}
	return nil
}

func (a *Manager) Close() {
	// do nothing for now
}
