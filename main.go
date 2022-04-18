package main

import (
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/joho/godotenv"
	"github.com/jonas747/dca"
)

var buffer = make([][]byte, 0)

var currentVoiceChannel *discordgo.VoiceConnection
var connST = false

func main() {

	// loadSound()

	err := godotenv.Load(".env")
	Token := os.Getenv("DISCORD_TOKEN")
	if err != nil {
		log.Fatal(err)
	}

	dg, err := discordgo.New("Bot " + Token)
	if err != nil {
		fmt.Println("Error creating a discord Session, ", err)
		os.Exit(1)
	}

	dg.AddHandler(ready)

	// Register messageCreate as a callback for the messageCreate events.
	dg.AddHandler(messageCreate)

	// Register guildCreate as a callback for the guildCreate events.
	dg.AddHandler(guildCreate)

	dg.AddHandler(playSound)

	err = dg.Open()
	if err != nil {
		fmt.Println("Error opening Discord Session, ", err)
		os.Exit(1)
	}
	fmt.Println("The bot is now running. Press CTRL-C to exit.")

	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt, os.Kill)
	<-sc

	dg.Close()
}

func ready(s *discordgo.Session, event *discordgo.Ready) {
	s.UpdateGameStatus(0, "buongiorno ragazzi")
}

// This function will be called (due to AddHandler above) every time a new
// message is created on any channel that the autenticated bot has access to.
func messageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {

	// Ignore all messages created by the bot itself
	// This isn't required in this specific example but it's a good practice.
	if m.Author.ID == s.State.User.ID {
		return
	}

	// check if the message is "!airhorn"
	if strings.HasPrefix(m.Content, "!join") {

		// Find the channel that the message came from.
		c, err := s.State.Channel(m.ChannelID)
		if err != nil {
			// Could not find channel.
			return
		}

		// Find the guild for that channel.
		g, err := s.State.Guild(c.GuildID)
		if err != nil {
			// Could not find guild.
			return
		}

		// Look for the message sender in that guild's current voice states.
		for _, vs := range g.VoiceStates {
			if vs.UserID == m.Author.ID {

				// Join the provided voice channel.
				currentVoiceChannel, err = s.ChannelVoiceJoin(g.ID, vs.ChannelID, false, true)
				if err != nil {
					fmt.Println("Error connecting to voicechat:", err)
				}

				connST = true

				return
			}
		}
	} else if strings.HasPrefix(m.Content, "!quit") {

		// Disconnect from the provided voice channel.
		currentVoiceChannel.Disconnect()
		connST = false
	}
}

// This function will be called (due to AddHandler above) every time a new
// guild is joined.
func guildCreate(s *discordgo.Session, event *discordgo.GuildCreate) {

	if event.Guild.Unavailable {
		return
	}

	for _, channel := range event.Guild.Channels {
		if channel.ID == event.Guild.ID {
			_, _ = s.ChannelMessageSend(channel.ID, "E' arrivato il masse!")
			return
		}
	}
}

// loadSound attempts to load an encoded sound file from disk.
func loadSound() error {

	file, err := os.Open("soundbite.dca")
	if err != nil {
		fmt.Println("Error opening dca file :", err)
		return err
	}

	var opuslen int16

	for {
		// Read opus frame length from dca file.
		err = binary.Read(file, binary.LittleEndian, &opuslen)

		// If this is the end of the file, just return.
		if err == io.EOF || err == io.ErrUnexpectedEOF {
			err := file.Close()
			if err != nil {
				return err
			}
			return nil
		}

		if err != nil {
			fmt.Println("Error reading from dca file :", err)
			return err
		}

		// Read encoded pcm from dca file.
		InBuf := make([]byte, opuslen)
		err = binary.Read(file, binary.LittleEndian, &InBuf)

		// Should not be any end of file errors
		if err != nil {
			fmt.Println("Error reading from dca file :", err)
			return err
		}

		// Append encoded pcm data to the buffer.
		buffer = append(buffer, InBuf)
	}
}

// playSound plays the current buffer to the provided channel.
func playSound(s *discordgo.Session, event *discordgo.VoiceStateUpdate) {

	// check if bot is not in any voice channel or the new person is not in his channel
	if currentVoiceChannel == nil || event.ChannelID != currentVoiceChannel.ChannelID {
		return
	}

	// Sleep for a specified amount of time before playing the sound
	time.Sleep(1 * time.Second)

	file, _ := os.Open("soundbite.dca")

	// inputReader is an io.Reader, like a file for example
	decoder := dca.NewDecoder(file)

	for {
		frame, err := decoder.OpusFrame()
		if err != nil {
			if err != io.EOF {
				// Handle the error
			}

			break
		}

		// Do something with the frame, in this example were sending it to discord
		select {
		case currentVoiceChannel.OpusSend <- frame:
		case <-time.After(time.Second):
			// We haven't been able to send a frame in a second, assume the connection is borked
			return
		}
	}

	// Start speaking.
	currentVoiceChannel.Speaking(true)

	// Send the buffer data.
	for _, buff := range buffer {
		currentVoiceChannel.OpusSend <- buff
	}

	// Stop speaking
	currentVoiceChannel.Speaking(false)

	return
}
