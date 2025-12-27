package agent

import (
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"time"

	"github.com/PlakarKorp/plakar/appcontext"
	"github.com/PlakarKorp/plakar/utils"
	"github.com/google/uuid"
	"github.com/vmihailenco/msgpack/v5"
)

type RequestPkt struct {
	Type string

	Secret      []byte
	RepoID      uuid.UUID
	StoreConfig map[string]string
}

type ResponsePkt struct {
	Err      string
	ExitCode int
}

type Client struct {
	conn net.Conn
	enc  *msgpack.Encoder
	dec  *msgpack.Decoder
}

var (
	ErrWrongVersion = errors.New("cached is running with a different version of plakar")
)

func RebuildStateFromCached(ctx *appcontext.AppContext, repoID uuid.UUID, storeConfig map[string]string) (int, error) {
	client, err := NewClient(filepath.Join(ctx.CacheDir, "cached.sock"), false)
	if err != nil {
		return 1, err
	}
	defer client.Close()

	go func() {
		<-ctx.Done()
		client.Close()
	}()

	req := &RequestPkt{
		Type:        "",
		Secret:      ctx.GetSecret(),
		RepoID:      repoID,
		StoreConfig: storeConfig,
	}

	if err := client.enc.Encode(req); err != nil {
		return 1, err
	}

	response := &ResponsePkt{}
	for {
		if err := client.dec.Decode(response); err != nil {
			if err == io.EOF {
				break
			}
			if err := ctx.Err(); err != nil {
				return 1, err
			}
			return 1, fmt.Errorf("failed to decode response: %w", err)
		}

		var err error
		if response.Err != "" {
			err = fmt.Errorf("%s", response.Err)
		}

		return response.ExitCode, err
	}

	return 0, nil
}

func NewClient(socketPath string, ignoreVersion bool) (*Client, error) {
	var lockfile *os.File
	var spawned bool

	defer func() {
		if lockfile != nil {
			lockfile.Close()
			os.Remove(lockfile.Name())
		}
	}()

	var (
		attempt int
		conn    net.Conn
		err     error
	)

	for {
		conn, err = net.Dial("unix", socketPath)
		if err == nil {
			// connected successfully!
			break
		}

		attempt++
		if attempt > 1000 {
			return nil, fmt.Errorf("failed to run the agent")
		}

		if lockfile == nil {
			lockfile, err = os.OpenFile(socketPath+".lock",
				os.O_WRONLY|os.O_CREATE, 0600)
			if err != nil {
				return nil, fmt.Errorf("failed to create lockfile: %w", err)
			}

			err = flock(lockfile)
			if err != nil {
				return nil, fmt.Errorf("failed to take the lock: %w", err)
			}

			// Always retry at least once, even if we got
			// the lock, because another client could have
			// taken the lock, started the server and
			// released the lock between our net.Dial and
			// unix.Flock.

			continue
		}

		if !spawned {
			me, err := os.Executable()
			if err != nil {
				return nil, fmt.Errorf("failed to get executable: %w", err)
			}

			plakar := exec.Command(me, "cached")
			if err := plakar.Start(); err != nil {
				return nil, fmt.Errorf("failed to start the agent: %w", err)
			}
			spawned = true
		}

		time.Sleep(5 * time.Millisecond)
	}

	encoder := msgpack.NewEncoder(conn)
	decoder := msgpack.NewDecoder(conn)

	c := &Client{
		conn: conn,
		enc:  encoder,
		dec:  decoder,
	}

	if err := c.handshake(ignoreVersion); err != nil {
		return nil, err
	}

	return c, nil
}

func (c *Client) handshake(ignoreVersion bool) error {
	ourvers := []byte(utils.GetVersion())

	if err := c.enc.Encode(ourvers); err != nil {
		return err
	}

	var agentvers []byte
	if err := c.dec.Decode(&agentvers); err != nil {
		return err
	}

	if !ignoreVersion && !slices.Equal(ourvers, agentvers) {
		return fmt.Errorf("%w (%v)", ErrWrongVersion, string(agentvers))
	}

	return nil
}

func (c *Client) Close() error {
	return c.conn.Close()
}
