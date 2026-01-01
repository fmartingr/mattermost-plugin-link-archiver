package main

import (
	"net/http"
	"sync"
	"time"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin"
	"github.com/mattermost/mattermost/server/public/pluginapi"
	"github.com/mattermost/mattermost/server/public/pluginapi/cluster"
	"github.com/pkg/errors"

	"github.com/fmartingrmattermost-plugin-link-archiver/server/command"
	"github.com/fmartingrmattermost-plugin-link-archiver/server/store/kvstore"
)

// Plugin implements the interface expected by the Mattermost server to communicate between the server and plugin processes.
type Plugin struct {
	plugin.MattermostPlugin

	// kvstore is the client used to read/write KV records for this plugin.
	kvstore kvstore.KVStore

	// client is the Mattermost server API client.
	client *pluginapi.Client

	// commandClient is the client used to register and execute slash commands.
	commandClient command.Command

	backgroundJob *cluster.Job

	// configurationLock synchronizes access to the configuration.
	configurationLock sync.RWMutex

	// configuration is the active plugin configuration. Consult getConfiguration and
	// setConfiguration for usage.
	configuration *configuration

	// archiveProcessor handles archival of URLs in posts
	archiveProcessor *ArchiveProcessor

	// botService manages the archiver bot account
	botService *BotService

	// threadReplyService handles creating thread replies
	threadReplyService *ThreadReplyService
}

// OnActivate is invoked when the plugin is activated. If an error is returned, the plugin will be deactivated.
func (p *Plugin) OnActivate() error {
	p.client = pluginapi.NewClient(p.API, p.Driver)

	p.kvstore = kvstore.NewKVStore(p.client)

	p.commandClient = command.NewCommandHandler(p.client)

	// Initialize bot service and ensure bot exists
	p.botService = NewBotService(p.API)
	if err := p.botService.EnsureBotExists(); err != nil {
		return errors.Wrap(err, "failed to ensure bot account exists")
	}

	// Initialize thread reply service
	p.threadReplyService = NewThreadReplyService(p.API, p.botService.GetBotID())

	// Initialize archive processor
	linkExtractor := NewLinkExtractor()
	contentDetector := NewContentDetector(10 * time.Second)
	storageService := NewStorageService(p.API)
	p.archiveProcessor = NewArchiveProcessor(p.API, linkExtractor, contentDetector, storageService, p.threadReplyService)

	job, err := cluster.Schedule(
		p.API,
		"BackgroundJob",
		cluster.MakeWaitForRoundedInterval(1*time.Hour),
		p.runJob,
	)
	if err != nil {
		return errors.Wrap(err, "failed to schedule background job")
	}

	p.backgroundJob = job

	return nil
}

// OnDeactivate is invoked when the plugin is deactivated.
func (p *Plugin) OnDeactivate() error {
	if p.backgroundJob != nil {
		if err := p.backgroundJob.Close(); err != nil {
			p.API.LogError("Failed to close background job", "err", err)
		}
	}
	return nil
}

// This will execute the commands that were registered in the NewCommandHandler function.
func (p *Plugin) ExecuteCommand(c *plugin.Context, args *model.CommandArgs) (*model.CommandResponse, *model.AppError) {
	response, err := p.commandClient.Handle(args)
	if err != nil {
		return nil, model.NewAppError("ExecuteCommand", "plugin.command.execute_command.app_error", nil, err.Error(), http.StatusInternalServerError)
	}
	return response, nil
}

// MessageHasBeenPosted is invoked when a message has been posted by a user.
// This hook is called after the message has been committed to the database.
func (p *Plugin) MessageHasBeenPosted(c *plugin.Context, post *model.Post) {
	// Ignore messages from the bot itself to prevent infinite loops
	if p.botService != nil && post.UserId == p.botService.GetBotID() {
		return
	}

	// Get current configuration
	config := p.getConfiguration()

	// Process the post for archival (async, non-blocking)
	go func() {
		if err := p.archiveProcessor.ProcessPost(post.Id, post.Message, config); err != nil {
			p.API.LogError("Failed to process post for archival", "postID", post.Id, "error", err.Error())
		}
	}()
}

// See https://developers.mattermost.com/extend/plugins/server/reference/
