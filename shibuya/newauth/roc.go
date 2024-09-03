package newauth

import (
	"strings"

	"github.com/MicahParks/keyfunc/v2" // We are using v2 because 1.18 does not support v3
	"github.com/golang-jwt/jwt/v5"
)

type ROCClaim struct {
	Groups []string `json:"groups"`
	jwt.RegisteredClaims
}

var _ AuthResult = (*ROCClaim)(nil)

func (roc *ROCClaim) GetUser() string {
	return roc.Subject
}

func (roc *ROCClaim) GetOrgs() []string {
	return roc.Groups
}

type (
	SubscriptionString string
	ServiceProvider    string
	UserRole           string
)

func (ss SubscriptionString) split() (ServiceProvider, UserRole) {
	t := strings.Split(string(ss), "::")
	// need to deal with this shit: rns:roc:registry-aas:jpw1:rmsj-rpage-dui-web-server-lb:projects:rmsj-rpage-dui-web-server-lb-default:roles:admin
	if len(t) < 2 {
		return "", ""
	}
	return ServiceProvider(t[0]), UserRole(t[1])
}

func (sp ServiceProvider) shortName() string {
	t := strings.Split(string(sp), ":")
	if len(t) < 3 {
		return ""
	}
	return t[2]
}

func (ur UserRole) shortName() string {
	t := strings.Split(string(ur), ":")
	if len(t) < 1 {
		return ""
	}
	return t[0]
}

func (ur UserRole) Role() string {
	t := strings.Split(string(ur), ":")
	if len(t) < 3 {
		return ""
	}
	return t[2]
}

type Role struct {
	Claim   AuthResult
	roleMap map[string]string
}

func NewRole(sp string, claim AuthResult) *Role {
	r := &Role{Claim: claim}
	r.roleMap = r.makeInternalMap(sp, r.Claim.GetOrgs())
	return r
}

func (r *Role) makeInternalMap(sp string, orgs []string) map[string]string {
	m := make(map[string]string)
	for _, org := range orgs {
		ss := SubscriptionString(org)
		spp, ur := ss.split()
		if spp.shortName() != sp {
			continue
		}
		if ur.Role() == "" {
			continue
		}
		m[ur.shortName()] = ur.Role()
	}
	return m
}

func (r *Role) IsSubscribed(self string) bool {
	for _, org := range r.Claim.GetOrgs() {
		ss := SubscriptionString(org)
		sp, _ := ss.split()
		if sp.shortName() == self {
			return true
		}
	}
	return false
}

func (r *Role) IsAdmin(user string) bool {
	t, ok := r.roleMap[user]
	if !ok {
		return false
	}
	return t == "admin"
}

var jwksURL = ""

func makeSigningFunc(jwksUrl string) (*keyfunc.JWKS, error) {
	jwks, err := keyfunc.Get(jwksUrl, keyfunc.Options{})
	if err != nil {
		return nil, err
	}
	return jwks, nil
}

func ROCIAMAuth(tokenString string) (AuthResult, error) {
	// jwks, err := keyfunc.Get(jwksURL, keyfunc.Options{})
	// if err != nil {
	// 	return nil, err
	// }
	token, err := ParseTokenWithKeyFunc(tokenString, &ROCClaim{}, nil)
	if err != nil {
		return nil, err
	}
	c := token.Claims.(*ROCClaim)
	return c, err
}
