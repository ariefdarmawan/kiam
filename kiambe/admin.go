package kiambe

import (
	"errors"
	"io"

	"git.kanosolution.net/kano/kaos"
	"github.com/ariefdarmawan/kiam/acm"
	"github.com/sebarcode/codekit"
)

type adminEngine struct {
	//km *acm.Manager
}

func NewAdminEngine() *adminEngine {
	ae := new(adminEngine)
	return ae
}

func (am *adminEngine) SaveUser(ctx *kaos.Context, usr *acm.User) (*acm.User, error) {
	h, e := ctx.DefaultHub()
	if e != nil {
		return nil, errors.New("invalid hub")
	}
	mgr := acm.NewACM(h)

	dbUsr, e := mgr.GetUser("ID", usr.ID)
	if e != nil && e == io.EOF {
		dbUsr, e = mgr.CreateUser(usr.LoginID, usr.Name, usr.Email, usr.Phone, codekit.RandomString(12))
		if e != nil {
			return nil, e
		}
		//return dbUsr, nil
	} else if e != nil {
		return nil, e
	}

	usr.ID = dbUsr.ID
	if e = h.Save(usr); e != nil {
		return nil, e
	}

	return usr, nil
}

type ChangePasswordRequest struct {
	ID, Password, ConfirmPassword string
}

func (am *adminEngine) ChangeUserPassword(ctx *kaos.Context, req *ChangePasswordRequest) (codekit.M, error) {
	res := codekit.M{}
	if req.Password != req.ConfirmPassword {
		return res, errors.New("password and confirm does not match")
	}

	h, e := ctx.DefaultHub()
	if e != nil {
		return res, errors.New("invalid hub")
	}
	mgr := acm.NewACM(h)

	dbUsr, e := mgr.GetUser("ID", req.ID)
	if e != nil {
		return res, e
	}
	if e = mgr.SetPassword(dbUsr, req.Password); e != nil {
		return res, e
	}
	return res.Set("Message", "OK"), nil
}
