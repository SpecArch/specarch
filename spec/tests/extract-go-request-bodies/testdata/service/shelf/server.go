package shelf

import (
	"github.com/gin-gonic/gin"

	"example.com/catalogue/pb"
)

// Server answers the shelf's routes.
type Server struct{}

// Engine makes the engine that serves the shelf.
func Engine() *gin.Engine {
	r := gin.New()
	s := &Server{}
	s.routes(r)
	return r
}

// routes registers the shelf's routes on the engine.
func (s *Server) routes(r *gin.Engine) {
	r.POST("/items", s.create)
	r.PUT("/items/:id", s.replace)
	r.POST("/imports", s.importItems)
	r.POST("/items/:id/policy", s.upsert)
	r.POST("/tags", s.tag)
}

// remark is a note on an item.
type remark struct {
	Text string `json:"text"`
}

// create decodes its body into a struct written in place, with a nested
// struct, a pointer to one, a slice of them, a named struct, a map and a
// field with no json tag.
func (s *Server) create(c *gin.Context) {
	var req struct {
		Name string `json:"name" binding:"required"`
		Size *struct {
			Width int32 `json:"width"`
		} `json:"size"`
		Tags []struct {
			Label string `json:"label"`
		} `json:"tags"`
		Note  remark            `json:"note"`
		Extra map[string]string `json:"extra"`
		Count int32
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	c.Status(201)
}

// replace names its variable body and binds with BindJSON.
func (s *Server) replace(c *gin.Context) {
	var body struct {
		Name string `json:"name"`
	}
	if err := c.BindJSON(&body); err != nil {
		return
	}
	_ = c.Param("id")
	c.Status(204)
}

// importItems decodes its body into a type of another module.
func (s *Server) importItems(c *gin.Context) {
	var req pb.Item
	if err := c.ShouldBindJSON(&req); err != nil {
		return
	}
	c.Status(202)
}

// upsertRequest is a named struct whose property is a map of any value.
type upsertRequest struct {
	Policy map[string]any `json:"policy"`
}

// upsert decodes its body into the named struct with the map.
func (s *Server) upsert(c *gin.Context) {
	var req upsertRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		return
	}
	_ = c.Param("id")
	c.Status(204)
}

// tag decodes its body into a struct whose only field is an interface.
func (s *Server) tag(c *gin.Context) {
	var req struct {
		Data any `json:"data"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		return
	}
	c.Status(204)
}
