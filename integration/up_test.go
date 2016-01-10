package integration

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/docker/libcompose/utils"
	dockerclient "github.com/fsouza/go-dockerclient"
	. "gopkg.in/check.v1"
)

func (s *RunSuite) TestUp(c *C) {
	p := s.ProjectFromText(c, "up", SimpleTemplate)

	name := fmt.Sprintf("%s_%s_1", p, "hello")
	cn := s.GetContainerByName(c, name)
	c.Assert(cn, NotNil)

	c.Assert(cn.State.Running, Equals, true)
}

func (s *RunSuite) TestUpNotExistService(c *C) {
	p := s.ProjectFromText(c, "up", SimpleTemplate)

	name := fmt.Sprintf("%s_%s_1", p, "not_exist")
	cn := s.GetContainerByName(c, name)
	c.Assert(cn, IsNil)
}

func (s *RunSuite) TestRecreateForceRecreate(c *C) {
	p := s.ProjectFromText(c, "up", SimpleTemplate)

	name := fmt.Sprintf("%s_%s_1", p, "hello")
	cn := s.GetContainerByName(c, name)
	c.Assert(cn, NotNil)

	p = s.FromText(c, p, "up", "--force-recreate", SimpleTemplate)
	cn2 := s.GetContainerByName(c, name)
	c.Assert(cn.ID, Not(Equals), cn2.ID)
}

func mountSet(slice []dockerclient.Mount) map[string]bool {
	result := map[string]bool{}
	for _, v := range slice {
		result[fmt.Sprint(v.Source, ":", v.Destination)] = true
	}
	return result
}

func (s *RunSuite) TestRecreateVols(c *C) {
	p := s.ProjectFromText(c, "up", SimpleTemplateWithVols)

	name := fmt.Sprintf("%s_%s_1", p, "hello")
	cn := s.GetContainerByName(c, name)
	c.Assert(cn, NotNil)

	p = s.FromText(c, p, "up", "--force-recreate", SimpleTemplateWithVols2)
	cn2 := s.GetContainerByName(c, name)
	c.Assert(cn.ID, Not(Equals), cn2.ID)

	notHomeRootOrVol2 := func(mount string) bool {
		switch strings.SplitN(mount, ":", 2)[1] {
		case "/home", "/root", "/var/lib/vol2":
			return false
		}
		return true
	}

	shouldMigrate := utils.FilterStringSet(mountSet(cn.Mounts), notHomeRootOrVol2)
	cn2Mounts := mountSet(cn2.Mounts)
	for k := range shouldMigrate {
		c.Assert(cn2Mounts[k], Equals, true)
	}

	almostTheSameButRoot := utils.FilterStringSet(cn2Mounts, notHomeRootOrVol2)
	c.Assert(len(almostTheSameButRoot), Equals, len(cn2Mounts)-1)
	c.Assert(cn2Mounts["/tmp/tmp-root:/root"], Equals, true)
	c.Assert(cn2Mounts["/root:/root"], Equals, false)
}

func (s *RunSuite) TestRecreateNoRecreate(c *C) {
	p := s.ProjectFromText(c, "up", SimpleTemplate)

	name := fmt.Sprintf("%s_%s_1", p, "hello")
	cn := s.GetContainerByName(c, name)
	c.Assert(cn, NotNil)

	p = s.FromText(c, p, "up", "--no-recreate", `
	hello:
	  labels:
	    key: val
	  image: busybox
	  stdin_open: true
	  tty: true
	`)
	cn2 := s.GetContainerByName(c, name)
	c.Assert(cn.ID, Equals, cn2.ID)
	_, ok := cn2.Config.Labels["key"]
	c.Assert(ok, Equals, false)
}

func (s *RunSuite) TestRecreate(c *C) {
	p := s.ProjectFromText(c, "up", SimpleTemplate)

	name := fmt.Sprintf("%s_%s_1", p, "hello")
	cn := s.GetContainerByName(c, name)
	c.Assert(cn, NotNil)

	p = s.FromText(c, p, "up", SimpleTemplate)
	cn2 := s.GetContainerByName(c, name)
	c.Assert(cn.ID, Equals, cn2.ID)

	p = s.FromText(c, p, "up", `
	hello:
	  labels:
	    key: val
	  image: busybox
	  stdin_open: true
	  tty: true
	`)
	cn3 := s.GetContainerByName(c, name)
	c.Assert(cn2.ID, Not(Equals), cn3.ID)
	key3 := cn3.Config.Labels["key"]
	c.Assert(key3, Equals, "val")

	// Should still recreate because old has a different label
	p = s.FromText(c, p, "up", `
	hello:
	  image: busybox
	  stdin_open: true
	  tty: true
	`)
	cn4 := s.GetContainerByName(c, name)
	c.Assert(cn3.ID, Not(Equals), cn4.ID)
	_, ok4 := cn4.Config.Labels["key"]
	c.Assert(ok4, Equals, false)

	p = s.FromText(c, p, "up", `
	hello:
	  image: busybox
	  stdin_open: true
	  tty: true
	`)
	cn5 := s.GetContainerByName(c, name)
	c.Assert(cn4.ID, Equals, cn5.ID)
	_, ok5 := cn5.Config.Labels["key"]
	c.Assert(ok5, Equals, false)

	p = s.FromText(c, p, "up", "--force-recreate", `
	hello:
	  image: busybox
	  stdin_open: true
	  tty: true
	`)
	cn6 := s.GetContainerByName(c, name)
	c.Assert(cn5.ID, Not(Equals), cn6.ID)
	_, ok6 := cn6.Config.Labels["key"]
	c.Assert(ok6, Equals, false)

	p = s.FromText(c, p, "up", "--force-recreate", `
	hello:
	  image: busybox
	  stdin_open: true
	  tty: true
	`)
	cn7 := s.GetContainerByName(c, name)
	c.Assert(cn6.ID, Not(Equals), cn7.ID)
	_, ok7 := cn7.Config.Labels["key"]
	c.Assert(ok7, Equals, false)

	c.Assert(cn.State.Running, Equals, true)
}

func (s *RunSuite) TestUpAfterImageTagDeleted(c *C) {
	client := GetClient(c)
	label := RandStr(7)
	repo := "busybox"
	image := fmt.Sprintf("%s:%s", repo, label)

	template := fmt.Sprintf(`
	hello:
	  labels:
	    key: val
	  image: %s
	  stdin_open: true
	  tty: true
	`, image)

	err := client.TagImage("busybox:latest", dockerclient.TagImageOptions{Repo: repo, Tag: label, Force: true})
	c.Assert(err, IsNil)

	p := s.ProjectFromText(c, "up", template)
	name := fmt.Sprintf("%s_%s_1", p, "hello")
	firstContainer := s.GetContainerByName(c, name)

	err = client.RemoveImage(image)
	c.Assert(err, IsNil)

	p = s.FromText(c, p, "up", "--no-recreate", template)
	latestContainer := s.GetContainerByName(c, name)
	c.Assert(firstContainer.ID, Equals, latestContainer.ID)
}

func (s *RunSuite) TestRecreateImageChanging(c *C) {
	client := GetClient(c)
	label := "buildroot-2013.08.1"
	repo := "busybox"
	image := fmt.Sprintf("%s:%s", repo, label)

	template := fmt.Sprintf(`
	hello:
	  labels:
	    key: val
	  image: %s
	  stdin_open: true
	  tty: true
	`, image)

	// Ignore error here
	client.RemoveImage(image)

	// Up, pull needed
	p := s.ProjectFromText(c, "up", template)
	name := fmt.Sprintf("%s_%s_1", p, "hello")
	firstContainer := s.GetContainerByName(c, name)

	// Up --no-recreate, no pull needed
	p = s.FromText(c, p, "up", "--no-recreate", template)
	latestContainer := s.GetContainerByName(c, name)
	c.Assert(firstContainer.ID, Equals, latestContainer.ID)

	// Up --no-recreate, no pull needed
	p = s.FromText(c, p, "up", "--no-recreate", template)
	latestContainer = s.GetContainerByName(c, name)
	c.Assert(firstContainer.ID, Equals, latestContainer.ID)

	// Change what tag points to
	err := client.TagImage("busybox:latest", dockerclient.TagImageOptions{Repo: repo, Tag: label, Force: true})
	c.Assert(err, IsNil)

	// Up (with recreate - the default), pull is needed and new container is created
	p = s.FromText(c, p, "up", template)
	latestContainer = s.GetContainerByName(c, name)
	c.Assert(firstContainer.ID, Not(Equals), latestContainer.ID)

	s.FromText(c, p, "rm", "-f", template)
}

func (s *RunSuite) TestLink(c *C) {
	p := s.ProjectFromText(c, "up", `
        server:
          image: busybox
          command: cat
          stdin_open: true
          expose:
          - 80
        client:
          image: busybox
          links:
          - server:foo
          - server
        `)

	serverName := fmt.Sprintf("%s_%s_1", p, "server")

	cn := s.GetContainerByName(c, serverName)
	c.Assert(cn, NotNil)
	c.Assert(cn.Config.ExposedPorts, DeepEquals, map[dockerclient.Port]struct{}{
		"80/tcp": {},
	})

	clientName := fmt.Sprintf("%s_%s_1", p, "client")
	cn = s.GetContainerByName(c, clientName)
	c.Assert(cn, NotNil)
	c.Assert(asMap(cn.HostConfig.Links), DeepEquals, asMap([]string{
		fmt.Sprintf("/%s:/%s/%s", serverName, clientName, "foo"),
		fmt.Sprintf("/%s:/%s/%s", serverName, clientName, "server"),
		fmt.Sprintf("/%s:/%s/%s", serverName, clientName, serverName),
	}))
}

func (s *RunSuite) TestRelativeVolume(c *C) {
	p := s.ProjectFromText(c, "up", `
	server:
	  image: busybox
	  volumes:
	    - .:/path
	`)

	absPath, err := filepath.Abs(".")
	c.Assert(err, IsNil)
	serverName := fmt.Sprintf("%s_%s_1", p, "server")
	cn := s.GetContainerByName(c, serverName)

	c.Assert(cn, NotNil)
	c.Assert(len(cn.Mounts), DeepEquals, 1)
	c.Assert(cn.Mounts[0].Source, DeepEquals, absPath)
}
