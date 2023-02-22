package worker

const dataMsgPreFix = "00"

const (
	md5CheckError = "File sync error. Md5 is different"
)

const (
	VARIANCE_ROOT = iota
	VARIANCE_ADD
	VARIANCE_EDIT
	VARIANCE_DELETE
)

type CapacityUnit int64

const (
	Byte CapacityUnit = 1
	KB                = 1024 * Byte
	MB                = 1024 * KB
	GB                = 1000 * MB
	TB                = 60 * GB
	PB                = 60 * TB
)

var fileSeparator = getFileSeparator()

type FileNode struct {
	IsDirectory      bool
	HasChildren      bool
	AbstractPath     string
	AnchorPointPath  string
	ChildrenNodeList []*FileNode
	HeadFileNode     *FileNode
	VarianceType     int
}

type SyncFileError struct {
	AbsPath string
	Reason  string
}

// LocalBSFileNode /** Local batch source file node
var LocalBSFileNode *FileNode

// LocalBTFileNode /** Local batch target file node
var LocalBTFileNode *FileNode

type Disk struct {
	Name       string
	Partitions []Partition
	MediaType  int
	Speed      uint64
}

type Partition struct {
	Name        string
	FsType      string
	TotalSize   string
	FreeSize    string
	UsedPercent float64
}

func getFileSeparator() string {
	//if strings.Contains(runtime.GOOS, "window") {
	//	return "\\"
	//} else if strings.Contains(runtime.GOOS, "linux") {
	//	return "/"
	//}
	return "/"
}
