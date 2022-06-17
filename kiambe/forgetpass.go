package kiambe

import (
	"errors"
	"time"

	"git.kanosolution.net/kano/dbflex"
	"git.kanosolution.net/kano/kaos"
	"github.com/ariefdarmawan/kiam"
	"github.com/ariefdarmawan/kiam/acm"
	"github.com/ariefdarmawan/kmsg"
	"github.com/sebarcode/codekit"
)

type ForgetPassOptions struct {
	SendTokenReminderTopic string
	MailFactory            kiam.MailFactory
}

type fpass struct {
	opts ForgetPassOptions
}

func NewForgetPass(opts ForgetPassOptions) *fpass {
	o := new(fpass)
	o.opts = opts
	return o
}

func (fp *fpass) SendToken(ctx *kaos.Context, email string) (string, error) {
	h, _ := ctx.DefaultHub()
	if h == nil {
		return "", errors.New("invalid datahub")
	}

	if email == "" {
		return "", errors.New("Email is mandatory")
	}

	user := new(acm.User)
	h.GetByParm(user, dbflex.NewQueryParam().SetWhere(dbflex.Eq("Email", email)))
	if user.ID == "" {
		return "", nil
	}

	// user is valid, send the reminder token
	token, e := acm.CreateToken(h, user.ID, "ForgetPassToken", 30)
	if e != nil {
		return "", errors.New("system error when generating token: " + e.Error())
	}

	var msg *kmsg.Message
	if fp.opts.MailFactory != nil {
		var tkm = map[string]interface{}{
			"token": token,
			"name":  user.Name,
			"email": user.Email,
			"url":   "/iam/forgetpasschange?token=" + token,
		}

		mm := fp.opts.MailFactory.MakeEmail("ForgotPassword", tkm)
		msg, e = kmsg.NewMessage(h, "ForgetPassToken", "SMTP", "", user.Email,
			mm.Subject,
			mm.Body)
		if e != nil {
			return "", errors.New("system error preparing token message: " + e.Error())
		}
	} else {
		msg, e = kmsg.NewMessage(h, "ForgetPassToken", "SMTP", "", user.Email,
			"Token untuk Pengingat Password",
			"Link untuk pengingat password adalah: /iam/forgetpasschange?token="+token)
		if e != nil {
			return "", errors.New("system error preparing token message: " + e.Error())
		}
	}

	ev, e := ctx.DefaultEvent()
	if ev == nil {
		return "", errors.New("invalid eventhub")
	}

	reply := ""
	if err := ev.Publish(fp.opts.SendTokenReminderTopic, msg.ID, &reply); err != nil {
		return "", errors.New("system error when sending token: " + err.Error())
	}

	return "", nil
}

func (fp *fpass) ChangePwd(ctx *kaos.Context, req codekit.M) (string, error) {
	h, _ := ctx.DefaultHub()
	if h == nil {
		return "", errors.New("invalid data hub")
	}

	email := req.GetString("Email")
	tid := req.GetString("Token")
	pass := req.GetString("Password")

	// gte token
	t := new(acm.Token)
	h.GetByID(t, tid)
	if t.ID == "" || t.Kind != "ForgetPassToken" || t.Status != "Claimed" {
		return "", errors.New("invalid token")
	}
	if time.Now().After(t.ClaimTime.Add(time.Duration(t.ValidDurationInMinute) * time.Minute)) {
		return "", errors.New("expired token")
	}

	u := new(acm.User)
	h.GetByAttr(u, "Email", email)
	if t.UserID != u.ID {
		return "", errors.New("invalid token")
	}

	acm := acm.NewACM(h)
	if e := acm.SetPassword(u, pass); e != nil {
		return "", errors.New("fail change password: " + e.Error())
	}

	t.Status = "Used"
	h.Save(t)

	return "OK", nil
}
