package main

import (
	"os"
	"path/filepath"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin"
	"github.com/pkg/errors"
)

const (
	// BotUsername is the username for the link-archiver bot
	BotUsername = "link-archiver"
	// BotDisplayName is the display name for the link-archiver bot
	BotDisplayName = "Link Archiver"
	// BotDescription is the description for the link-archiver bot
	BotDescription = "Automatically archives links posted to channels"
)

// BotService manages the archiver bot account
type BotService struct {
	api     plugin.API
	botID   string
	botUser *model.User
}

// NewBotService creates a new bot service
func NewBotService(api plugin.API) *BotService {
	return &BotService{
		api: api,
	}
}

// EnsureBotExists ensures the bot account exists, creating it if necessary
func (b *BotService) EnsureBotExists() error {
	// Try to get existing bot user
	user, appErr := b.api.GetUserByUsername(BotUsername)
	if appErr == nil && user != nil {
		// Bot already exists
		b.botID = user.Id
		b.botUser = user
		// Set bot profile image from plugin assets (in case it wasn't set before)
		if err := b.setBotProfileImage(); err != nil {
			// Log error but don't fail activation if profile image can't be set
			b.api.LogWarn("Failed to set bot profile image", "error", err.Error())
		}
		return nil
	}

	// Bot doesn't exist, create it
	botUser := &model.User{
		Username:            BotUsername,
		FirstName:           BotDisplayName,
		LastName:            "",
		Email:               BotUsername + "@localhost",
		Password:            model.NewId(),
		Nickname:            BotDisplayName,
		Position:            BotDescription,
		Roles:               model.SystemUserRoleId,
		Locale:              "en",
		DisableWelcomeEmail: true,
	}

	createdUser, appErr := b.api.CreateUser(botUser)
	if appErr != nil {
		return errors.Wrap(appErr, "failed to create bot user")
	}

	b.botID = createdUser.Id
	b.botUser = createdUser

	// Set bot profile image from plugin assets
	if err := b.setBotProfileImage(); err != nil {
		// Log error but don't fail activation if profile image can't be set
		b.api.LogWarn("Failed to set bot profile image", "error", err.Error())
	}

	return nil
}

// setBotProfileImage sets the bot's profile image from the plugin's icon asset
func (b *BotService) setBotProfileImage() error {
	// Get the plugin bundle path
	bundlePath, err := b.api.GetBundlePath()
	if err != nil {
		return errors.Wrap(err, "failed to get bundle path")
	}

	// Read the icon file
	iconPath := filepath.Join(bundlePath, "assets", "icon.png")
	iconData, err := os.ReadFile(iconPath)
	if err != nil {
		return errors.Wrap(err, "failed to read icon file")
	}

	// Set the profile image for the bot
	if appErr := b.api.SetProfileImage(b.botID, iconData); appErr != nil {
		return errors.Wrap(appErr, "failed to set profile image")
	}

	return nil
}

// GetBotUser returns the bot user
func (b *BotService) GetBotUser() *model.User {
	return b.botUser
}

// GetBotID returns the bot user ID
func (b *BotService) GetBotID() string {
	return b.botID
}
