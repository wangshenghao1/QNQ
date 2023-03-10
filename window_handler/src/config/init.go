package config

import (
	"os"
	"runtime"
	"time"
)

const CLI_FALG = false

var LocalSystemInfo = systemInfo{
	OS:              runtime.GOOS,
	SystemFramework: runtime.GOARCH,
	MachineName:     getLocalMachineName(),
}

var TargetSystemInfo = systemInfo{
	OS:              GET_INFO_FAILURE,
	SystemFramework: GET_INFO_FAILURE,
	MachineName:     GET_INFO_FAILURE,
}

func init() {
	loadInitConfigCache()
	_, err := os.Open(CONFOG_PATH)
	if err != nil {
		filePtr, _ := os.Create(CONFOG_PATH)
		addObserver()
		loadDefaultConfig()
		defer func() {
			filePtr.Close()
		}()
		return
	}
	addObserver()
	loadConfig()
}

// TODO 配置新增后的版本升级处理
func loadDefaultConfig() {
	defaultSyncPolicy := syncPolicy{
		PolicySwitch: false,
		PeriodicSync: periodicSyncPolicy{
			Cycle:  time.Hour,
			Rate:   1,
			Enable: false,
		},
		TimingSync: timingSyncPolicy{
			Days:   [7]bool{false, false, false, false, false, false, false},
			Hour:   15,
			Minute: 0,
			Enable: false,
		},
	}
	defaultLocal := localSync{
		SourcePath: NOT_SET_STR,
		TargetPath: NOT_SET_STR,
		SyncPolicy: defaultSyncPolicy,
		Speed:      NOT_SET_STR,
		CheckMd5:   false,
	}
	defaultRemote := qnqTarget{
		Ip:         "0.0.0.0",
		LocalPath:  NOT_SET_STR,
		RemotePath: NOT_SET_STR,
		SyncPolicy: defaultSyncPolicy,
	}

	defaultConfig := systemConfig{
		Version:         version,
		QnqSTarget:      defaultRemote,
		QnqBTarget:      defaultRemote,
		LocalSingleSync: defaultLocal,
		LocalBatchSync:  defaultLocal,
		PartitionSync:   defaultLocal,
		VarianceAnalysis: varianceAnalysis{
			TimeStamp: true,
			Md5:       true,
		},
	}
	SystemConfigCache.Set(defaultConfig)
}
