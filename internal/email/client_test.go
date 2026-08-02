package email

import (
	"context"
	"testing"

	"nutmeg/internal/assert"
)

func TestBuildMessage(t *testing.T) {
	msg := string(buildMessage("noreply@nutmeg.local", []string{"a@x.com", "b@x.com"}, "Hello", "Body text"))

	assert.StrContains(t, msg, "From: noreply@nutmeg.local")
	assert.StrContains(t, msg, "To: a@x.com, b@x.com")
	assert.StrContains(t, msg, "Subject: Hello")
	assert.StrContains(t, msg, "Body text")
}

func TestSend_NoOpWhenUnconfigured(t *testing.T) {
	c := NewClient("", "", "", "", "noreply@nutmeg.local")
	err := c.Send(context.Background(), []string{"a@x.com"}, "Hello", "Body")
	assert.NoErr(t, err)
}
