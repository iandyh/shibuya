package auth

import (
	"errors"
	"fmt"
	"regexp"

	"github.com/rakutentech/shibuya/shibuya/config"
	ldap "gopkg.in/ldap.v2"
)

var (
	CNPattern  = regexp.MustCompile(`CN=([^,]+)\,OU=DLM\sDistribution\sGroups`)
	AccountKey = "account"
	MLKey      = "ml"
)

type LdapAuth struct {
	config.LdapConfig
}

func NewLdapAuth[C config.InputBackendConf](c C) InputRequiredAuth {
	return LdapAuth{
		LdapConfig: config.LdapConfig(*c),
	}
}

func (la LdapAuth) ValidateInput(username, password string) (AuthResult, error) {
	r := AuthResult{}
	ldapServer := la.LdapServer
	ldapPort := la.LdapPort
	baseDN := la.BaseDN
	r.ML = []string{}

	filter := "(&(objectClass=user)(sAMAccountName=%s))"
	l, err := ldap.Dial("tcp", fmt.Sprintf("%s:%s", ldapServer, ldapPort))
	if err != nil {
		return r, err
	}
	defer l.Close()
	systemUser := la.SystemUser
	systemPassword := la.SystemPassword
	err = l.Bind(systemUser, systemPassword)
	if err != nil {
		return r, err
	}
	searchRequest := ldap.NewSearchRequest(
		baseDN,
		ldap.ScopeWholeSubtree, ldap.NeverDerefAliases, 0, 0, false,
		fmt.Sprintf(filter, username),
		[]string{"userprincipalname"},
		nil,
	)
	sr, err := l.Search(searchRequest)
	if err != nil {
		return r, err
	}

	entries := sr.Entries
	if len(entries) != 1 {
		return r, errors.New("Users does not exist")
	}

	attributes := entries[0].Attributes
	if len(attributes) == 0 {
		return r, errors.New("Cannot find the user")
	}
	values := attributes[0].Values
	if len(values) == 0 {
		return r, errors.New("Cannot find the principle name")
	}
	UserPrincipalName := values[0]
	err = l.Bind(UserPrincipalName, password)
	if err != nil {
		return r, errors.New("Incorrect password")
	}
	searchRequest = ldap.NewSearchRequest(
		baseDN,
		ldap.ScopeWholeSubtree, ldap.NeverDerefAliases, 0, 0, false,
		fmt.Sprintf(filter, username),
		[]string{"memberOf"},
		nil,
	)
	sr, err = l.Search(searchRequest)
	if err != nil {
		return r, errors.New("Error in contacting LDAP server")
	}
	entries = sr.Entries
	if len(entries) == 0 {
		return r, errors.New("Cannot find user ml/group information")
	}
	attributes = entries[0].Attributes
	if len(attributes) == 0 {
		return r, errors.New("Cannot find user group/ml information")
	}
	values = attributes[0].Values
	for _, m := range values {
		match := CNPattern.FindStringSubmatch(m)
		if match == nil {
			continue
		}
		r.ML = append(r.ML, match[1])
	}
	return r, nil
}
