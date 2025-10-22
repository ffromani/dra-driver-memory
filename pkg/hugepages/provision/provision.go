package provision

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"

	"github.com/go-logr/logr"
	ghwopt "github.com/jaypipes/ghw/pkg/option"
	ghwtopology "github.com/jaypipes/ghw/pkg/topology"

	"sigs.k8s.io/yaml"

	apiv0 "github.com/ffromani/dra-driver-memory/pkg/hugepages/provision/api/v0"
)

func ReadConfiguration(source string) (apiv0.HugePageProvision, error) {
	if source == "-" {
		return readConfigurationFrom(os.Stdin)
	}
	src, err := os.Open(source)
	if err != nil {
		return apiv0.HugePageProvision{}, err
	}
	//nolint:errcheck
	defer src.Close()
	return readConfigurationFrom(src)
}

func RuntimeHugepages(logger logr.Logger, hpp apiv0.HugePageProvision, sysRoot string) error {
	logger.V(2).Info("start provisioning hugepages", "groups", len(hpp.Spec.Pages))
	defer logger.V(2).Info("done provisioning hugepages", "groups", len(hpp.Spec.Pages))

	sysinfo, err := ghwtopology.New(ghwopt.WithChroot(sysRoot))
	if err != nil {
		return err
	}

	for _, conf := range hpp.Spec.Pages {
		var err error

		if len(sysinfo.Nodes) == 1 {
			numaNode := 0
			if conf.Node != nil {
				numaNode = int(*conf.Node)
			}
			logger.V(2).Info("provisioning pages", "numaNode", numaNode, "count", conf.Count, "size", conf.Size)
			err = provisionOnNode(logger, numaNode, int(conf.Count), conf.Size, sysRoot)
		} else {
			logger.V(2).Info("splitting pages", "count", conf.Count, "NUMACount", len(sysinfo.Nodes))
			err = provisionOnMultiNode(logger, len(sysinfo.Nodes), int(conf.Count), conf.Size, sysRoot)
		}

		if err != nil {
			return err
		}
	}
	return nil
}

func provisionOnMultiNode(logger logr.Logger, numaNodeCount, hpCount int, hpSize apiv0.HugePageSize, sysRoot string) error {
	extra := hpCount % numaNodeCount
	perNode := hpCount / numaNodeCount

	// we choose to move excess pages on numa node 0 because this is the most common observed practice
	err := provisionOnNode(logger, 0, perNode+extra, hpSize, sysRoot)
	if err != nil {
		return err
	}
	for numaNode := 1; numaNode < numaNodeCount; numaNode++ {
		err = provisionOnNode(logger, numaNode, perNode, hpSize, sysRoot)
		if err != nil {
			return err
		}
	}
	return nil
}

func provisionOnNode(logger logr.Logger, numaNode, hpCount int, apiHpSize apiv0.HugePageSize, sysRoot string) error {
	// this is done too late, we should have proper validation and API translation but good enough for starters.
	hpSize, err := apiv0.ValidateHugePageSize(apiHpSize)
	if err != nil {
		return err
	}
	hpPath := filepath.Join(sysRoot, "sys", "devices", "system", "node", fmt.Sprintf("node%d", numaNode), "hugepages", "hugepages-"+hpSize, "nr_hugepages")
	logger.V(4).Info("writing on sysfs", "path", hpPath)
	dst, err := os.OpenFile(hpPath, os.O_WRONLY, 0)
	if err != nil {
		return err
	}
	//nolint:errcheck
	defer dst.Close()
	_, err = dst.WriteString(strconv.Itoa(hpCount))
	if err != nil {
		return fmt.Errorf("failed to write on %q: %w", hpPath, err)
	}
	return err
}

func readConfigurationFrom(r io.Reader) (apiv0.HugePageProvision, error) {
	hpp := apiv0.HugePageProvision{}
	data, err := io.ReadAll(r)
	if err != nil {
		return hpp, err
	}
	err = yaml.Unmarshal(data, &hpp)
	return hpp, err
}
