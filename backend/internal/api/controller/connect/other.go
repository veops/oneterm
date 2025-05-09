package connect

import (
	"bufio"
	"bytes"
	"database/sql"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"github.com/samber/lo"
	"github.com/spf13/cast"
	mysqlDriver "gorm.io/driver/mysql"
	"gorm.io/gorm"

	"github.com/veops/oneterm/internal/model"
	gsession "github.com/veops/oneterm/internal/session"
	"github.com/veops/oneterm/internal/tunneling"
)

// connectOther connects to other protocols (MySQL, Redis, etc.)
func connectOther(ctx *gin.Context, sess *gsession.Session, asset *model.Asset, account *model.Account, gateway *model.Gateway) (err error) {
	chs := sess.Chans
	defer func() {
		if err != nil {
			chs.ErrChan <- err
		}
	}()

	protocol := strings.Split(sess.Protocol, ":")[0]
	ip, port, err := tunneling.Proxy(false, sess.SessionId, protocol, asset, gateway)
	if err != nil {
		return
	}

	var (
		rdb *redis.Client
		db  *gorm.DB
	)
	switch protocol {
	case "redis":
		rdb = redis.NewClient(&redis.Options{
			Addr:        fmt.Sprintf("%s:%d", ip, port),
			Password:    account.Password,
			DialTimeout: time.Second,
		})
		_, err = rdb.Ping(ctx).Result()
		if err != nil {
			return
		}
	case "mysql":
		dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/?charset=utf8mb4&parseTime=True&loc=Local", account.Account, account.Password, ip, port)
		db, err = gorm.Open(mysqlDriver.Open(dsn))
		if err != nil {
			return
		}
	}

	chs.ErrChan <- err

	sess.G.Go(func() error {
		reader := bufio.NewReader(chs.Rin)
		buf := &bytes.Buffer{}
		pt := ""
		ss := strings.Split(sess.Protocol, ":")
		if len(ss) == 2 {
			pt = ss[1]
		}
		sess.Prompt = fmt.Sprintf("%s@%s:%s> ", account.Account, asset.Name, pt)
		chs.OutChan <- append(byteRN, []byte(sess.Prompt)...)
		for {
			select {
			case <-sess.Gctx.Done():
				return nil
			default:
				rn, size, err := reader.ReadRune()
				if err != nil {
					return err
				}
				if size <= 0 || rn == utf8.RuneError {
					continue
				}
				p := make([]byte, utf8.RuneLen(rn))
				utf8.EncodeRune(p, rn)
				p = bytes.ReplaceAll(p, byteT, byteS)
				for bytes.HasSuffix(p, byteDel) {
					p = p[:len(p)-1]
					if buf.Len() > 0 {
						var dels []byte
						last, ok := lo.Last([]rune(buf.String()))
						for i := 0; ok && i < lipgloss.Width(string(last)); i++ {
							dels = append(dels, byteClearCur...)
						}
						chs.OutChan <- dels
						buf.Truncate(buf.Len() - len([]byte(string(last))))
					}
				}
				if len(p) <= 0 {
					continue
				}
				chs.OutChan <- p
				buf.Write(p)
				bs := buf.Bytes()
				if idx := bytes.LastIndex(bs, byteClearAll); idx >= 0 {
					buf.Reset()
					continue
				}
				if idx := bytes.LastIndex(bs, byteR); idx < 0 {
					continue
				}
				bs = bs[:len(bs)-1]
				if bytes.Equal(bs, []byte("exit")) {
					sess.Once.Do(func() { close(chs.AwayChan) })
					return nil
				}
				buf.Reset()
				var (
					res  any
					rows *sql.Rows
				)
				if len(bs) > 0 {
					switch protocol {
					case "redis":
						parts := lo.Map(reRedis.FindAllString(string(bs), -1), func(p string, _ int) any { return p })
						res, err = rdb.Do(ctx, parts...).Result()
					case "mysql":
						if rows, err = db.WithContext(ctx).Raw(string(bs)).Rows(); err == nil {
							heads, _ := rows.Columns()
							n := len(heads)
							rs := make([][]string, 0)
							for rows.Next() {
								r := make([]any, n)
								r = lo.Map(r, func(v any, _ int) any { return new(any) })
								if err = rows.Scan(r...); err != nil {
									break
								}
								rs = append(rs, lo.Map(r, func(v any, i int) string { return cast.ToString(v) }))
							}
							res = strings.ReplaceAll(table.New().Border(border).Headers(heads...).Rows(rs...).String(), "\n", "\r\n")
						}
					}
				}
				chs.OutChan <- []byte(fmt.Sprintf("\n%s\r\n%s", lo.Ternary[any](err == nil, lo.Ternary(res == nil, "", res), err), sess.Prompt))
				err = nil
			}
		}
	})
	sess.G.Go(func() (err error) {
		for {
			select {
			case <-sess.Gctx.Done():
				return
			case <-chs.AwayChan:
				return
			case <-chs.WindowChan:
				continue
			}
		}
	})

	sess.G.Wait()

	return
}
