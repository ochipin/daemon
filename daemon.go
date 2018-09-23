package daemon

import (
	"fmt"
	"io"
	"io/ioutil"
	"log/syslog"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"sync"
	"syscall"
	"time"
)

const (
	// DaemonStarting : 起動開始
	DaemonStarting = "STARTING"
	// DaemonSuccess : 起動成功
	DaemonSuccess = "SUCCESS"
	// NotStartedDaemon : デーモン起動をまだしていない状態
	NotStartedDaemon = ""
	// StartingDaemon : デーモン起動開始
	StartingDaemon = "0"
	// StartedDaemon : デーモン起動完了
	StartedDaemon = "1"
)

// New : Daemon構造体を初期化する
func New() (*Daemon, error) {
	// アプリケーション名を取得
	_, appname := filepath.Split(os.Args[0])
	// デフォルトでは、出力先をシスログとする
	logger, err := syslog.New(syslog.LOG_ALERT|syslog.LOG_USER, appname)
	if err != nil {
		return nil, err
	}
	// 初期設定のみを施し、呼び出し側へ返却する
	return &Daemon{
		StartWait:  1000,
		ErrorWait:  500,
		WorkingDir: "/",
		Appname:    appname,
		Envname:    "__DAEMON_GOLANG__",
		Cmdpath:    os.Args[0],
		Pidfile:    "/tmp/" + appname + ".pid",
		Stdout:     os.Stdout,
		Stderr:     os.Stderr,
		Writer:     logger,
		Exec: func() error {
			return fmt.Errorf("%s: daemon.Exec() failed. not implemented yet", appname)
		},
	}, nil
}

// Daemon : デーモン起動に必要な情報を取り扱う構造体
type Daemon struct {
	StartWait  int          // 起動待ち時間
	ErrorWait  int          // エラー発生待ち時間
	WorkingDir string       // ワーキングディレクトリ
	Appname    string       // アプリケーション名
	Envname    string       // デーモンの起動状態を管理する環境変数名
	Cmdpath    string       // 起動するコマンド名
	Pidfile    string       // PIDファイルのパス
	Stdout     io.Writer    // 標準出力IO
	Stderr     io.Writer    // 標準エラー出力IO
	Writer     io.Writer    // エラーが発生した際に出力するIO
	Exec       func() error // デーモン起動時にコールされる
}

// Stat : PID
func (daemon *Daemon) Stat() error {
	// Pidfile が未指定の場合、エラーを返却する
	if daemon.Pidfile == "" {
		return fmt.Errorf("pidfile path is not set")
	}

	// Pidfile のファイル情報を取得する
	f, err := os.Stat(daemon.Pidfile)
	// Pidfile が存在しなかった場合は作成する
	if err != nil {
		// プロセスIDを取得する
		pid := fmt.Sprint(os.Getpid())
		// ディレクトリが存在しない場合、ディレクトリを作成する
		if dir, _ := filepath.Split(daemon.Pidfile); dir != "" {
			os.MkdirAll(dir, 0755)
		}
		// 権限等で、書き込めない場合はエラーとする
		if err = ioutil.WriteFile(daemon.Pidfile, []byte(pid), 0600); err != nil {
			return err
		}
		return nil
	}

	// 既にPidfileは存在するが、ディレクトリの場合はエラーとする
	if f.IsDir() {
		return fmt.Errorf("'%s' is a directory. already exists", f.Name())
	}

	// Pidfileが存在している場合、Pidfileを読み込む
	buf, _ := ioutil.ReadFile(daemon.Pidfile)
	// Pidfileの内容が、数字に変換できない場合はエラーとする
	pidnum, err := strconv.Atoi(string(buf))
	if err != nil {
		return fmt.Errorf("'%s' already exists", f.Name())
	}
	// プロセスIDが存在している場合、エラーとする
	if syscall.Kill(pidnum, syscall.Signal(0)) == nil {
		return fmt.Errorf("'%s(%d) process already exists", f.Name(), pidnum)
	}
	// プロセスIDが存在していない場合、現在のプロセスIDでPidfileを更新する
	pid := fmt.Sprint(os.Getpid())
	if err = ioutil.WriteFile(daemon.Pidfile, []byte(pid), 0600); err != nil {
		return err
	}
	return nil
}

// StartProc : プロセスを起動する
func (daemon *Daemon) StartProc(options []string) error {
	// 子プロセスとの通信用パイプ
	r, w, err := os.Pipe()
	if err != nil {
		return err
	}

	// コマンドを起動する
	cmd := exec.Command(daemon.Cmdpath, options...)
	cmd.ExtraFiles = []*os.File{w}
	cmd.Stdout = daemon.Stdout
	cmd.Stderr = daemon.Stderr
	cmd.Env = os.Environ()
	cmd.SysProcAttr = nil
	// 起動失敗時はエラーを返却
	if err := cmd.Start(); err != nil {
		return err
	}

	// パイプから、子プロセスの起動状態を取得
	var status = DaemonStarting
	go func() {
		buf := make([]byte, 1024)
		r.Read(buf)
		var str string
		for i := 0; i < len(buf); i++ {
			if int(buf[i]) != 0 {
				continue
			}
			str = string(buf[:i])
			break
		}
		if str == DaemonSuccess {
			status = DaemonSuccess
		} else {
			status = str
		}
	}()

	// 子プロセスの起動を待つ
	for i := 0; i < daemon.StartWait; i++ {
		// SUCCESS/FAILEDの場合ループを抜ける
		if status != DaemonStarting {
			break
		}
		// 1ms 秒単位で待つ
		time.Sleep(1 * time.Millisecond)
	}

	// 子プロセス起動失敗の場合は、エラーを返却する
	if status != DaemonSuccess {
		if status == DaemonStarting {
			status = "timeout. failed start daemon"
		}
		return fmt.Errorf("%s", status)
	}
	return nil
}

// Pipeline : 起動するプロセスとのメッセージのやり取りを行う
func (daemon *Daemon) Pipeline(err error) {
	pipe := os.NewFile(uintptr(3), "pipe")
	if pipe != nil {
		defer pipe.Close()
		// エラー発生時は、pipeにdaemonFailedを渡して復帰する
		if err != nil {
			pipe.Write([]byte(err.Error()))
			return
		}
		// 起動成功時は、pipeにDaemonSuccessを渡す
		pipe.Write([]byte(DaemonSuccess))
	}

	// SIGCHLDを無効化
	signal.Ignore(syscall.SIGCHLD)
	// STDIN/STDOUT/STDERRをクローズ
	syscall.Close(0)
	syscall.Close(1)
	syscall.Close(2)
	// プロセスグループのリーダになる
	syscall.Setsid()
	// umaskを022にセット
	syscall.Umask(022)
	// カレントディレクトリパスを移動
	syscall.Chdir(daemon.WorkingDir)
}

// MySelf : デーモン起動後に実行される関数
func (daemon *Daemon) MySelf() error {
	// デーモン起動後、PIDファイルが作成されているか確認する
	if err := daemon.Stat(); err != nil {
		daemon.Pipeline(err)
		return err
		// os.Exit(2)
	}

	// 別スレッドでCmdpathを起動する
	var err error
	var wg = &sync.WaitGroup{}
	go func() {
		wg.Add(1)
		err = daemon.Exec()
		wg.Done()
	}()

	// エラーがあるか待つ。エラーが発生した場合はプログラムを終了する
	for i := 0; i < daemon.ErrorWait; i++ {
		if err != nil {
			daemon.Pipeline(err)
			os.Remove(daemon.Pidfile)
			return err
			// os.Exit(2)
		}
		time.Sleep(1 * time.Millisecond)
	}

	// 起動成功の場合、環境変数に起動完了状態をセットする
	os.Setenv(daemon.Envname, StartedDaemon)
	daemon.Pipeline(nil)
	// スレッドの終了を待つ
	wg.Wait()
	// PIDファイルが残っている場合削除する
	os.Remove(daemon.Pidfile)
	// スレッド終了後、エラーが発生した場合、ログへ情報を出力する
	if err != nil {
		fmt.Fprintf(daemon.Writer, "%s\n", err)
		return err
	}

	return nil
}

// Daemon : デーモン起動開始
func (daemon *Daemon) Daemon(options []string) error {
	// デーモン起動をまだしていない場合
	if os.Getenv(daemon.Envname) == NotStartedDaemon {
		// デーモン起動中であることを環境変数を用いて管理する
		os.Setenv(daemon.Envname, StartingDaemon)
		// 指定されたコマンドを実行する
		if err := daemon.StartProc(options); err != nil {
			// fmt.Fprintf(daemon.Stderr, "%s\n", string(err.Error()))
			return err
		}
		return nil
	}

	// 自分自身起動の場合
	if daemon.Cmdpath == os.Args[0] {
		daemon.MySelf()
	}

	return nil
}
