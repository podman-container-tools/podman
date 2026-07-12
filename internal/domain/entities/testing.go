//go:build !remote

package entities

type CreateStorageLayerOptions struct {
	Parent          string
	ID              string
	ContentsArchive []byte
}

type CreateStorageLayerReport struct {
	ID string
}

type CreateLayerOptions struct {
	Parent string
	ID     string
}

type CreateLayerReport struct {
	ID string
}

type CreateLayerDataOptions struct {
	ID   string
	Data map[string][]byte
}

type CreateLayerDataReport struct{}

type CreateImageOptions struct {
	Layer string
	Names []string
	ID    string
}

type CreateImageReport struct {
	ID string
}

type CreateImageDataOptions struct {
	ID   string
	Data map[string][]byte
}

type CreateImageDataReport struct{}

type CreateContainerOptions struct {
	Layer string
	Image string
	Names []string
	ID    string
}

type CreateContainerReport struct {
	ID string
}

type CreateContainerDataOptions struct {
	ID   string
	Data map[string][]byte
}

type CreateContainerDataReport struct{}

type ModifyLayerOptions struct {
	ID              string
	ContentsArchive []byte
}

type ModifyLayerReport struct{}

type PopulateLayerOptions struct {
	ID              string
	ContentsArchive []byte
}

type PopulateLayerReport struct{}

type RemoveStorageLayerOptions struct {
	ID string
}

type RemoveStorageLayerReport struct {
	ID string
}

type RemoveLayerOptions struct {
	ID string
}

type RemoveLayerReport struct {
	ID string
}

type RemoveImageOptions struct {
	ID string
}

type RemoveImageReport struct {
	ID string
}

type RemoveContainerOptions struct {
	ID string
}

type RemoveContainerReport struct {
	ID string
}

type RemoveLayerDataOptions struct {
	ID  string
	Key string
}

type RemoveLayerDataReport struct{}

type RemoveImageDataOptions struct {
	ID  string
	Key string
}

type RemoveImageDataReport struct{}

type RemoveContainerDataOptions struct {
	ID  string
	Key string
}

type RemoveContainerDataReport struct{}

type ModifyLayerDataOptions struct {
	ID   string
	Key  string
	Data []byte
}

type ModifyLayerDataReport struct{}

type ModifyImageDataOptions struct {
	ID   string
	Key  string
	Data []byte
}

type ModifyImageDataReport struct{}

type ModifyContainerDataOptions struct {
	ID   string
	Key  string
	Data []byte
}

type ModifyContainerDataReport struct{}
