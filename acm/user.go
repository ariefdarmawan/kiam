package acm

import (
	"fmt"
	"time"

	"git.kanosolution.net/kano/dbflex"
	dbf "git.kanosolution.net/kano/dbflex"
	"git.kanosolution.net/kano/dbflex/orm"
	"github.com/sebarcode/codekit"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type User struct {
	orm.DataModelBase `bson:"-" json:"-" ecname:"-"`
	ID                string    `bson:"_id" json:"_id" key:"1" grid-show:"include" form-show:"hide"`
	LoginID           string    `search:"1" kf-pos:"2,1"`
	Name              string    `search:"1" kf-pos:"1,1"`
	Email             string    `search:"1" kf-pos:"2,2"`
	Phone             string    `search:"1" kf-pos:"3,1"`
	Kind              string    `kf-pos:"3,2"`
	Enable            bool      `kf-pos:"4,1"`
	Created           time.Time `grid-show:"hide" kf-pos:"10,1" readonly:"1"`
	LastUpdate        time.Time `kf-pos:"10,2" readonly:"1"`
}

func (u *User) TableName() string {
	return "ACMUsers"
}

func (u *User) SetID(keys ...interface{}) {
	u.ID = keys[0].(string)
}

func (u *User) PreSave(conn dbf.IConnection) error {
	if u.Created.Year() <= 1900 {
		u.Created = time.Now()
	}
	u.LastUpdate = time.Now()
	return nil
}

func (m *manager) GetUser(kind, name string) (*User, error) {
	var w *dbf.Filter
	if kind == "ID" {
		w = dbf.Eq("_id", name)
	} else if kind == "LoginID" {
		w = dbf.Eq("LoginID", name)
	}
	u := new(User)
	err := m.h.GetByParm(u, dbf.NewQueryParam().SetWhere(w))
	if err != nil {
		return nil, err
	}
	return u, nil
}

func (m *manager) CreateUser(loginid, name, email, phone, password string) (*User, error) {
	var e error

	// check for existing user
	var w *dbf.Filter
	if loginid == "" {
		w = dbf.Eq("Email", email)
	} else {
		w = dbf.Or(dbf.Eq("LoginID", loginid), dbf.Eq("Email", email))
	}
	u := new(User)

	m.h.GetByParm(u, dbf.NewQueryParam().SetWhere(w).SetTake(1))
	if u.ID != "" {
		return nil, fmt.Errorf("UserExist")
	}

	u.ID = primitive.NewObjectID().Hex()
	if loginid == "" {
		u.LoginID = u.ID
	} else {
		u.LoginID = loginid
	}
	u.Email = email
	u.Phone = phone
	u.Name = name
	u.Enable = true

	if e = m.h.Save(u); e != nil {
		return nil, fmt.Errorf("fail to save user: %s", e.Error())
	}

	if e = m.SetPassword(u, password); e != nil {
		return nil, fmt.Errorf("fail to save password: %s", e.Error())
	}

	return u, nil
}

func (mgr *manager) SetPassword(user *User, password string) error {
	passwd := new(passwrd)
	passwd.ID = user.ID
	passwd.Password = codekit.MD5String(password)
	return mgr.h.Save(passwd)
}

func (mgr *manager) ChangePassword(user *User, oldPass, newPass string) error {
	passwd := new(passwrd)
	passwd.ID = user.ID
	err := mgr.h.Get(passwd)

	if err != nil {
		return err
	}

	if passwd.Password != codekit.MD5String(oldPass) {
		return fmt.Errorf("invalid credentials to change password")
	}

	passwd.Password = codekit.MD5String(newPass)
	return mgr.h.Save(passwd)
}

func (mgr *manager) Authenticate(userid string, password string) (string, error) {
	user := new(User)
	if err := mgr.h.
		GetByParm(user, dbf.NewQueryParam().
			SetWhere(dbf.Or(
				dbflex.Eq("LoginID", userid),
				dbflex.Eq("Email", userid),
				dbflex.Eq("Phone", userid)))); err != nil {
		return "", fmt.Errorf(err.Error())
	}

	passwd := new(passwrd)
	passwd.ID = user.ID
	if err := mgr.h.Get(passwd); err != nil {
		return "", fmt.Errorf("InvalidCredentials2")
	}

	if passwd.Password != codekit.MD5String(password) {
		return "", fmt.Errorf("InvalidCredentials3")
	}

	if !user.Enable {
		return "", fmt.Errorf("User is disabled")
	}
	return user.ID, nil
}

func (m *manager) UserGroups(uid string, f *dbflex.Filter, sortBy string, lastIndex string, take int) ([]Group, error) {
	res := []Group{}
	if sortBy == "" {
		sortBy = "_id"
	}

	var w *dbflex.Filter
	if lastIndex != "" {
		w = dbflex.Lt(sortBy, lastIndex)
	} else {
		w = dbflex.And(dbflex.Lt(sortBy, lastIndex), f)
	}
	if f != nil {
		w = dbf.And(f, w)
	}

	if take == 0 {
		take = 20
	}

	cmd := dbflex.From(new(Group).TableName()).OrderBy(sortBy).Where(w)
	if take > 0 {
		cmd.Take(take)
	}

	if _, e := m.h.Populate(cmd, &res); e != nil {
		return res, e
	}

	return res, nil
}
