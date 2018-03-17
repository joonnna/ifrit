package main

// ClientConfig contains global server configurations for the Ifrit Certificate Authority daemon.
var ClientConfig = struct {
	Name            string `default:"Ifrit Client"`
	Version         string `default:"1.0.0"`
	Host            string `default:""`
	Port            string `default:"8080"`
	KeyFile         string `default:"./ca_key.pem"`
	CertificateFile string `default:"./ca_cert.pem"`
	LogFile         string

	CertificateAuthority struct {
		Host string
		Port string
	}
}{}
