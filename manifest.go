package main

import (
	"encoding/xml"
	"fmt"
	"net/url"
	"os"
)

type Manifest struct {
	XMLName  string    `xml:"manifest"`
	Defaults Default   `xml:"default"`
	Remotes  []Remote  `xml:"remote"`
	Projects []Project `xml:"project"`
}

type Remote struct {
	Name  string `xml:"name,attr"`
	Fetch string `xml:"fetch,attr"`
}

type Default struct {
	Revision string   `xml:"revision,attr"`
	Remote   string   `xml:"remote,attr"`
	Others   []string `xml:",any,attr"`
}

type Project struct {
	Name      string     `xml:"name,attr"`
	Path      string     `xml:"path,attr"`
	Remote    string     `xml:"remote,attr"`
	Revision  string     `xml:"revision,attr"`
	Copyfiles []Copyfile `xml:"copyfile"`
	Linkfiles []Linkfile `xml:"linkfile"`
}

type Linkfile struct {
	Src  string `xml:"src,attr"`
	Dest string `xml:"dest,attr"`
}

type Copyfile struct {
	Src  string `xml:"src,attr"`
	Dest string `xml:"dest,attr"`
}

func LoadManifest(filePath string) (manifest *Manifest, err error) {
	f, err := os.Open(filePath)
	if err != nil {
		err = fmt.Errorf("Fail to open file: %s", err)
		return
	}
	defer f.Close()

	decoder := xml.NewDecoder(f)

	var m Manifest
	err = decoder.Decode(&m)
	if err != nil {
		err = fmt.Errorf("Fail to parse xml: %s", err)
		return
	}

	manifest = &m

	return
}

func (m *Manifest) GetRevision(p *Project) (string, error) {
	rev := p.Revision
	if rev == "" {
		rev = m.Defaults.Revision
	}
	if rev == "" {
		return "", fmt.Errorf("No revision is specified, nor default revision is found")
	}

	return rev, nil
}

func (m *Manifest) GetRemote(p *Project) (string, string, error) {
	remoteName := p.Remote
	if remoteName == "" {
		remoteName = m.Defaults.Remote
	}
	if remoteName == "" {
		return "", "", fmt.Errorf("No remote is specified, nor default remote name is found")
	}

	for _, r := range m.Remotes {
		if r.Name == remoteName {
			remoteUrl, _ := url.JoinPath(r.Fetch, p.Name)
			return remoteName, remoteUrl, nil
		}
	}

	return "", "", fmt.Errorf("No specified remote is found")
}
