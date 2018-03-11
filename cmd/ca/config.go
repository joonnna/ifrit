package main

// CAConfig contains global server configurations for the Ifrit Certificate Authority daemon.
var CAConfig = struct {
	Name            string `default:"Ifrit CA"`
	Author          string
	Homepage        string
	Version         string `default:"1.0.0"`
	Host            string `default:""`
	Port            string `default:"8080"`
	KeyFile         string `default:"./ca_key.pem"`
	CertificateFile string `default:"./ca_cert.pem"`
	NumRings        uint   `default:"10"`
}{}
