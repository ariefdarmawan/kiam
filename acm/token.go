package acm

import (
	"errors"
	"fmt"
	"time"

	"git.kanosolution.net/kano/dbflex"
	"git.kanosolution.net/kano/dbflex/orm"
	"github.com/ariefdarmawan/datahub"
	"github.com/sebarcode/codekit"
)

type Token struct {
	orm.DataModelBase     `bson:"-" json:"-"`
	ID                    string `bson:"_id" json:"_id"`
	UserID                string
	Kind                  string
	CreateTime            time.Time
	ValidDurationInMinute int
	Status                string
	ClaimTime             time.Time
}

func (g *Token) TableName() string {
	return "ACMTokens"
}

func (g *Token) GetID(_ dbflex.IConnection) ([]string, []interface{}) {
	return []string{"_id"}, []interface{}{g.ID}
}

func (g *Token) SetID(keys ...interface{}) {
	if len(keys) > 0 {
		g.ID = keys[0].(string)
	}
}

func CreateToken(h *datahub.Hub, userID, kind string, validDurationInMinute int) (string, error) {
	token := codekit.GenerateRandomString("ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghiklmnopqrstuvwxyz0123456789", 32)
	tkn := new(Token)
	tkn.ID = token
	tkn.UserID = userID
	tkn.Kind = kind
	tkn.Status = "Open"
	tkn.CreateTime = time.Now()
	if validDurationInMinute == 0 {
		validDurationInMinute = 30
	}
	tkn.ValidDurationInMinute = validDurationInMinute
	if e := h.Save(tkn); e != nil {
		return "", errors.New("fail generate token: " + e.Error())
	}
	return token, nil
}

func ClaimToken(h *datahub.Hub, userID, kind, tokenID string) error {
	tkn := new(Token)
	tkn.ID = tokenID
	h.Get(tkn)
	if tkn.Kind != kind && tkn.UserID != userID {
		return errors.New("invalid token")
	}

	if tkn.Status != "Open" {
		return fmt.Errorf("invaid token")
	}

	if time.Now().After(tkn.CreateTime.Add(time.Duration(tkn.ValidDurationInMinute) * time.Minute)) {
		tkn.Status = "Expired"
		h.Save(tkn)
		return fmt.Errorf("token has been expired")
	}

	tkn.Status = "Claimed"
	tkn.ClaimTime = time.Now()
	h.Save(tkn)
	return nil
}

func UpdateTokenJob(h *datahub.Hub, kind string, checkPeriod time.Duration) chan bool {
	chanStopJob := make(chan bool)
	if int(checkPeriod) == 0 || checkPeriod < 5*time.Minute {
		checkPeriod = 5 * time.Minute
	}

	go func() {
		for {
			select {
			case <-chanStopJob:
				return

			case <-time.After(checkPeriod):
				w := dbflex.And(dbflex.Eq("Status", "Open"), dbflex.Eq("Kind", kind))
				tokens := []*Token{}
				h.PopulateByParm(new(Token).TableName(), dbflex.NewQueryParam().SetWhere(w), &tokens)
				for _, tkn := range tokens {
					if time.Now().After(tkn.CreateTime.Add(time.Duration(tkn.ValidDurationInMinute) * time.Minute)) {
						tkn.Status = "Expired"
						h.Save(tkn)
					}
				}
			}
		}
	}()

	return chanStopJob
}
