package worker

import (
	"io"
	"os"
	"time"
	"window_handler/common"
	"window_handler/config"
)

// TODO 及时释放这些引用
var totalSizeMap = make(map[string]uint64)
var doneSizeMap = make(map[string]uint64)
var batchErrMap = make(map[string][]string)

var batchSyncErrorCache = make([]SyncFileError, 0)
var (
	periodicLocalBatchTicker   *time.Ticker
	periodicLocalSingleTicker  *time.Ticker
	periodicRemoteBatchTicker  *time.Ticker
	periodicRemoteSingleTicker *time.Ticker
	timingLocalBatchTicker     *time.Ticker
	timingLocalSingleTicker    *time.Ticker
	timingRemoteBatchTicker    *time.Ticker
	timingRemoteSingleTicker   *time.Ticker
)

func NewLocalSingleWorker(sourceFile *os.File, targetFile *os.File, sn string) *common.QWorker {
	return &common.QWorker{
		Sub:             nil,
		ExecuteFunc:     LocalSyncSingleFile,
		DeconstructFunc: closeAndCheckFile,
		PrivateFile:     sourceFile,
		TargetFile:      targetFile,
	}
}

func FileNode2TreeMap(data *map[string][]string) {
	datat := *data
	datat[""] = append(datat[""], LocalBSFileNode.AbstractPath)
	GetFileTreeMap(LocalBSFileNode, data)
}

func GetFileTreeMap(node *FileNode, data *map[string][]string) {
	datat := *data
	if !node.IsDirectory {
		return
	}
	for _, child := range node.ChildrenNodeList {
		var key string
		if node.VarianceType == VARIANCE_ROOT {
			key = node.AbstractPath
		} else {
			key = node.AnchorPointPath
		}
		datat[key] = append(datat[key], child.AnchorPointPath)
		GetFileTreeMap(child, data)
	}
}

func batchSyncFile(startPath string, targetPath string, sn *string) {
	CreateDir(targetPath)
	sf, err := OpenDir(startPath)
	if err != nil {
		return
	}
	children, _ := sf.Readdir(-1)
	CloseFile(sf)
	ReverseCompareAndDelete(startPath, targetPath)
	for _, child := range children {
		targetAbsPath := targetPath + fileSeparator + child.Name()
		sourceAbsPath := startPath + fileSeparator + child.Name()
		if !child.IsDir() {
			tf, errT := OpenFile(targetAbsPath, true)
			rsf, errS := OpenFile(sourceAbsPath, true)
			if CompareMd5(tf, rsf) {
				fInfo, _ := tf.Stat()
				addSizeToDoneMap(*sn, uint64(fInfo.Size()))
				CloseFile(tf, rsf)
				continue
			}
			//reopen file
			CloseFile(tf, rsf)
			tf, errT = OpenFile(targetAbsPath, true)
			rsf, errS = OpenFile(sourceAbsPath, true)
			//common.GetCoroutinesPool().Submit(worker.Execute())
			if errT == nil && errS == nil {
				worker := NewLocalSingleWorker(rsf, tf, *sn)
				common.GetCoroutinesPool().Submit(worker.Execute)
			} else {
				CloseFile(rsf, tf)
			}
		} else {
			batchSyncFile(sourceAbsPath, targetAbsPath, sn)
		}
	}
}

// LocalBatchSyncOneTime 直接读取配置文件，无需参数
func LocalBatchSyncOneTime() {
	common.SendSignal2GWChannel(common.LOCAL_BATCH_POLICY_RUNNING)
	if common.LocalBatchPolicyRunningFlag {
		return
	}
	InitFileNode(true, false)
	StartLocalBatchSync()
	common.SendSignal2GWChannel(common.LOCAL_BATCH_POLICY_STOP)
}

// LocalSingleSyncOneTime 直接读取配置文件，无需参数
func LocalSingleSyncOneTime() {
	common.SendSignal2GWChannel(common.LOCAL_SINGLE_POLICY_RUNNING)
	sf, _ := OpenFile(config.SystemConfigCache.Cache.LocalSingleSync.SourcePath, false)
	tf := getSingleTargetFile(sf, config.SystemConfigCache.Cache.LocalSingleSync.TargetPath)
	worker := NewLocalSingleWorker(sf, tf, common.GetSNCount())
	common.GetCoroutinesPool().Submit(worker.Execute)
	common.SendSignal2GWChannel(common.LOCAL_SINGLE_POLICY_STOP)
}

func periodicLocalBatchSync() {
	if config.SystemConfigCache.Cache.LocalBatchSync.SyncPolicy.PeriodicSync.Enable {
		if config.SystemConfigCache.Value().LocalBatchSync.SyncPolicy.PolicySwitch {
			LocalBatchSyncOneTime()
		}
	}
}

func periodicLocalSingleSync() {
	if config.SystemConfigCache.Cache.LocalSingleSync.SyncPolicy.PeriodicSync.Enable {
		if config.SystemConfigCache.Cache.LocalSingleSync.SyncPolicy.PolicySwitch {
			LocalSingleSyncOneTime()
		}
	}
}

func periodicRemoteBatchSync() {
}

func periodicRemoteSingleSync() {
}

func timingLocalBatchSync() {
	if config.SystemConfigCache.Cache.LocalBatchSync.SyncPolicy.TimingSync.Enable {
		if config.SystemConfigCache.Cache.LocalBatchSync.SyncPolicy.PolicySwitch {
			LocalBatchSyncOneTime()
		}
	}

	if config.SystemConfigCache.Cache.LocalBatchSync.SyncPolicy.TimingSync.Enable {
		if config.SystemConfigCache.Cache.LocalBatchSync.SyncPolicy.PolicySwitch {
			nextTime := getNextTimeFromConfig(true, false)
			if nextTime == 0 {
				time.Sleep(61 * time.Second)
			}
			nextTime = getNextTimeFromConfig(true, false)
			notEnd := false
			StartPolicySync(nextTime, &notEnd, true, false, false)
		}
	}

}

func timingLocalSingleSync() {
	if config.SystemConfigCache.Cache.LocalSingleSync.SyncPolicy.TimingSync.Enable {
		LocalSingleSyncOneTime()
	}

	if config.SystemConfigCache.Cache.LocalSingleSync.SyncPolicy.TimingSync.Enable {
		nextTime := getNextTimeFromConfig(false, false)
		notEnd := false
		StartPolicySync(nextTime, &notEnd, false, false, false)
	}
}

func timingRemoteBatchSync() {
}

func timingRemoteSingleSync() {
}

func tickerWorker(ticker *time.Ticker, duration time.Duration, notEnd *bool, workerFunc func()) {
	ticker = time.NewTicker(duration)
	for {
		select {
		case <-ticker.C:
			if !*notEnd {
				go workerFunc()
			} else {
				return
			}
		}
	}
}

func StartPolicySync(duration time.Duration, notEnd *bool, isBatch bool, isRemote bool, isPeriodic bool) {
	ticker, workerFunc := getTicker(isBatch, isRemote, isPeriodic)
	if ticker != nil {
		ticker.Stop()
	}
	go tickerWorker(ticker, duration, notEnd, workerFunc)
}

func StartLocalSingleSync() bool {
	sf, err := OpenFile(config.SystemConfigCache.Value().LocalSingleSync.SourcePath, false)
	if err != nil {
		return false
	}
	tf := getSingleTargetFile(sf, config.SystemConfigCache.Value().LocalSingleSync.TargetPath)
	sn := common.GetSNCount()
	worker := NewLocalSingleWorker(sf, tf, sn)
	common.GetCoroutinesPool().Submit(worker.Execute)
	return true
}

func StartPartitionSync() {
	sn := common.GetSNCount()
	common.CurrentLocalPartSN = sn
	addSizeToDoneMap(sn, 1)
	getTotalSize(&sn, config.SystemConfigCache.Cache.PartitionSync.SourcePath)
	common.LocalPartStartLock.Unlock()
	batchSyncFile(
		config.SystemConfigCache.Cache.PartitionSync.SourcePath,
		config.SystemConfigCache.Cache.PartitionSync.TargetPath,
		&sn,
	)
}

func StartLocalBatchSync() {
	sn := common.GetSNCount()
	common.CurrentLocalBatchSN = sn
	addSizeToDoneMap(sn, 1)
	getTotalSize(&sn, config.SystemConfigCache.Cache.LocalBatchSync.SourcePath)
	common.LocalBatchStartLock.Unlock()
	batchSyncFile(
		config.SystemConfigCache.Cache.LocalBatchSync.SourcePath,
		config.SystemConfigCache.Cache.LocalBatchSync.TargetPath,
		&sn,
	)
}

func getSingleTargetFile(sf *os.File, targetPath string) *os.File {
	tempTarget := ""
	tf, err := OpenFile(config.SystemConfigCache.Value().LocalSingleSync.TargetPath, true)

	if err != nil {
		if IsOpenDirError(err, config.SystemConfigCache.Value().LocalSingleSync.TargetPath) {
			sfInfo, _ := sf.Stat()
			tempTarget = config.SystemConfigCache.Value().LocalSingleSync.TargetPath + "/" + sfInfo.Name()
			tf, err = OpenFile(tempTarget, true)
			if err != nil {
				return tf
			}
		} else {
			return nil
		}
	}
	return tf
}

func LocalSyncSingleFile(msg interface{}, q *common.QWorker) {
	buf := make([]byte, 4096*2)
	for {
		n, err := q.PrivateFile.Read(buf)
		if err != nil && err != io.EOF {
			return
		}
		if n == 0 {
			break
		}
		_, err = q.TargetFile.Write(buf[:n])
		if err != nil {
			sendNameToErrMap(q.SN, q.TargetFile.Name())
			break
		} else {
			addSizeToDoneMap(q.SN, uint64(n))
		}
	}
}

func closeAndCheckFile(w *common.QWorker) {
	if !CompareMd5(w.PrivateFile, w.TargetFile) {
		AddBatchSyncError(w.PrivateFile.Name(), md5CheckError)
	}
	CloseFile(w.TargetFile)
	CloseFile(w.PrivateFile)
}

func GetLocalBatchProgress(sn string) float64 {
	return float64(doneSizeMap[sn]) / float64(totalSizeMap[sn])
}

func GetBatchSyncError() []SyncFileError {
	defer func() {
		batchSyncErrorCache = make([]SyncFileError, 0)
	}()
	return batchSyncErrorCache
}

func AddBatchSyncError(absPath string, reason string) {
	node := SyncFileError{
		AbsPath: absPath,
		Reason:  reason,
	}
	batchSyncErrorCache = append(batchSyncErrorCache, node)
}

func getTicker(isBatch bool, isRemote bool, isPeriodic bool) (*time.Ticker, func()) {
	if isPeriodic {
		if isRemote {
			if isBatch {
				return periodicRemoteBatchTicker, periodicRemoteBatchSync
			}
			return periodicRemoteSingleTicker, periodicRemoteSingleSync
		}
		if isBatch {
			return periodicLocalBatchTicker, periodicLocalBatchSync
		}
		return periodicLocalSingleTicker, periodicLocalSingleSync
	} else {
		if isRemote {
			if isBatch {
				return timingRemoteBatchTicker, timingRemoteBatchSync
			}
			return timingRemoteSingleTicker, timingRemoteSingleSync
		}
		if isBatch {
			return timingLocalBatchTicker, timingLocalBatchSync
		}
		return timingLocalSingleTicker, timingLocalSingleSync
	}
}

func addSizeToDoneMap(sn string, size uint64) {
	doneSizeMap[sn] = doneSizeMap[sn] + size
}

func addSizeToTotalMap(sn string, size uint64) {
	totalSizeMap[sn] = totalSizeMap[sn] + size
}

func sendNameToErrMap(sn string, name string) {
	batchErrMap[sn] = append(batchErrMap[sn], name)
}
