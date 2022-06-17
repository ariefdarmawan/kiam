package kiambe

import (
	"errors"
	"net/http"
	"time"

	"git.kanosolution.net/kano/kaos"
	"github.com/ariefdarmawan/kiam"
	"github.com/ariefdarmawan/kiam/acm"
	"github.com/dgrijalva/jwt-go"
	"github.com/sebarcode/codekit"
)

type loginEngine struct {
	LoginID        string `kf-pos:"1,1" required:"true" label:"Login ID, email atau mobile no"`
	Password       string `kf-pos:"2,1" required:"true" label:"Password" kf-control:"password"`
	RememberMe     bool   `kf-pos:"3,1"`
	SecondLifeTime int    `form-show:"hide" grid-show:"hide"`

	im                 *kiam.Manager
	fnPostAuthenticate func(u *acm.User, m *codekit.M)
}

func NewLoginEngine(im *kiam.Manager) *loginEngine {
	l := new(loginEngine)
	l.im = im
	return l
}

func (l *loginEngine) SetFnPostAuthenticate(fn func(u *acm.User, m *codekit.M)) *loginEngine {
	l.fnPostAuthenticate = fn
	return l
}

/* need to do:
taken care of allowed number of multi session, currently has no control on it
*/
func (l *loginEngine) Authenticate(ctx *kaos.Context, req *loginEngine) (codekit.M, error) {
	signMtd := l.im.Options().SignMethod
	signKey := []byte(l.im.Options().SignSecret)

	h, e := ctx.DefaultHub()
	if e != nil {
		return codekit.M{}, errors.New("invalid hub")
	}

	// auth
	var (
		uid string
	)
	mgr := acm.NewACM(h)
	if req == nil {
		hr := ctx.Data().Get("http-request", new(http.Request)).(*http.Request)
		loginID, password, ok := hr.BasicAuth()
		if !ok {
			return codekit.M{}, errors.New("unable to get auth info")
		}
		uid, e = mgr.Authenticate(loginID, password)
		if e != nil {
			return codekit.M{}, e
		}
	} else {
		uid, e = mgr.Authenticate(req.LoginID, req.Password)
		if e != nil {
			return codekit.M{}, e
		}
	}

	usr, _ := mgr.GetUser("ID", uid)
	tokenData := codekit.M{}
	if l.fnPostAuthenticate != nil {
		l.fnPostAuthenticate(usr, &tokenData)
	}

	// setup jwt
	token, err := l.im.FindOrCreate(ctx, codekit.M{}.Set("ID", uid).Set("Duration", 0), tokenData)
	if err != nil {
		return codekit.M{}, err
	}
	// jwt-ed
	secondLifeTime := l.SecondLifeTime
	if secondLifeTime == 0 {
		secondLifeTime = l.im.Options().SecondLifeTime
	}
	if secondLifeTime == 0 {
		secondLifeTime = 60 * 60 * 24 //1 day
	}
	bc := new(jwt.StandardClaims)
	bc.Id = token.SessionID
	bc.ExpiresAt = time.Now().Add(time.Duration(secondLifeTime) * time.Second).Unix()
	jtkn := jwt.NewWithClaims(signMtd, bc)
	tknString, err := jtkn.SignedString(signKey)
	if err != nil {
		return codekit.M{}, err
	}

	res := codekit.M{}
	for k, v := range tokenData {
		res.Set(k, v)
	}
	res.Set("Token", tknString).Set("Name", usr.Name)
	return res, nil
}

func (l *loginEngine) Logout(ctx *kaos.Context, req codekit.M) (string, error) {
	tokenid := ctx.Data().Get("jwt-token-id", "").(string)
	if tokenid == "" {
		return "token not found", nil
	}

	l.im.Remove(ctx, codekit.M{}.Set("ID", tokenid))
	return "token has been logged-out", nil
}
