package manager

import (
	"context"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/telepresenceio/telepresence/v2/cmd/traffic/cmd/manager/internal/cluster"
	"gopkg.in/square/go-jose.v2/jwt"
)

type license struct {
	license string
	host    string
	pubKeys map[string]string
}

func newLicenseFromDisk(rootDir string) (*license, error) {
	var l license

	buf, err := os.ReadFile(filepath.Join(rootDir, "license"))
	if err != nil {
		return nil, fmt.Errorf("error reading license: %w", err)
	}
	if l.license = string(buf); l.license == "" {
		return nil, fmt.Errorf("license is empty")
	}

	buf, err = os.ReadFile(filepath.Join(rootDir, "hostDomain"))
	if err != nil {
		return nil, fmt.Errorf("error reading hostDomain: %w", err)
	}
	if l.host = string(buf); l.host == "" {
		return nil, fmt.Errorf("host domain is empty")
	}

	return &l, nil
}

func (l *license) getClaims(ctx context.Context) (*licenseClaims, error) {
	if l == nil {
		return nil, fmt.Errorf("no license available")
	}

	if len(l.pubKeys) == 0 {
		l.pubKeys = pubKeys
	}

	hostKey, ok := l.pubKeys[l.host]
	if !ok {
		return nil, fmt.Errorf("unknown host")
	}

	block, _ := pem.Decode([]byte(hostKey))
	if block == nil {
		return nil, fmt.Errorf("no PEM data found for public key")
	}

	pubKey, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, err
	}

	token, err := jwt.ParseSigned(l.license)
	if err != nil {
		return nil, fmt.Errorf("failed to parse license: %w", err)
	}

	var claims licenseClaims
	return &claims, token.Claims(pubKey, &claims)
}

type licenseClaims struct {
	jwt.Claims
	AccountID string      `json:"accountId"`
	Limits    interface{} `json:"limits"`
	Scope     string      `json:"scope"`
}

func (lc *licenseClaims) isValidForCluster(ci cluster.Info) (bool, error) {
	expiry := lc.Expiry
	if expiry != nil && time.Now().After(expiry.Time()) {
		return false, errors.New("license has expired")
	}

	cid := ci.GetClusterID()
	claims := lc.Claims
	if !claims.Audience.Contains(cid) {
		return false, fmt.Errorf("license is for cluster(s) with these UIDs: %v. This cluster has ID: %s", claims.Audience, cid)
	}

	return true, nil
}

var pubKeys = map[string]string{
	"beta-auth.datawire.io": `-----BEGIN PUBLIC KEY-----
MIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEA15qWmyHoAE2Voqg91Ugh
hVUfQPYofd3eYOqpatWsILnNy68DtOSO/JWAao0YE63aBUHnSe08gGaVEZuWaQH3
jg7E5pvnAMwEHsDFegKR08Z4nGTkAMIR3SSr63nroMCEeTRFW0TWb3zDlk3u4tAE
zVsdui2mGIMnbYNYsiKE5988KWOhRf6OjAGldA+dIgS5vnEClocoyQNKlTME2qHL
4FMKgsaitLzrOMPZH2zHbf/AK6/KdJmCTBZlHH2zEMMnOXaw1Oe3SHubHax9KYi5
CaGJ+ividI7W6cwy90CtdAUObEVW+5KscNupltt9PyUJN69F0wPCY5yjSQCar27p
5QIDAQAB
-----END PUBLIC KEY-----`,
	"auth.datawire.io": `-----BEGIN PUBLIC KEY-----
MIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEAzDwrd/nO5ofA0WoH8NYv
Y0XX3SzYq6BmSxM3/P4ZZBvW35il8hBWv9T2cUPZDFdw77aOo/dhEXqiqtrG49kT
iZgNXe787q0wHqerUzLpT0ojPVE1iHLatVROcG+qQcBHJX+2+9NBRin6wh3dDAJU
tPh/yUWDVNSWO/sCObBAwHL8O/lbgVUboN40eESefOmMWLr0GJ9wNd63t9dkq5ue
/xu9HSPlWB3UaSz1vP5uByuX8gFH5G8uCG8Km8Qh4hObiSgkuJwdO4iBF/VeYYNh
EtZipbj7iCRMzkMo2QQfMz58V1G9I1kuC6+xKyKBUxtfUDuEDCgyC66ig35iGChg
uwIDAQAB
-----END PUBLIC KEY-----`,
}
