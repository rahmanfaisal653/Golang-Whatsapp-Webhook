package owner

import (
	"context"
	"fmt"
	"strings"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/types/events"
)

type GroupsCommand struct{ publicCommand }

func (GroupsCommand) Name() string { return ".groups" }

func (GroupsCommand) Execute(ctx context.Context, client *whatsmeow.Client, msg *events.Message) error {
	groups, err := client.GetJoinedGroups(ctx)
	if err != nil {
		return fmt.Errorf("get joined groups: %w", err)
	}
	if len(groups) == 0 {
		return reply(ctx, client, msg, "No joined groups found.")
	}

	lines := []string{"Joined groups:"}
	for _, group := range groups {
		lines = append(lines, fmt.Sprintf("%s\n%s", group.Name, group.JID))
	}
	return reply(ctx, client, msg, strings.Join(lines, "\n\n"))
}
