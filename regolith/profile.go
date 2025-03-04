package regolith

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"github.com/otiai10/copy"
)

// SetupTmpFiles set up the workspace for the filters.
func SetupTmpFiles(config Config, profile Profile) error {
	start := time.Now()
	// Setup Directories
	Logger.Debug("Cleaning .regolith/tmp")
	err := os.RemoveAll(".regolith/tmp")
	if err != nil {
		return err
	}

	err = os.MkdirAll(".regolith/tmp", 0666)
	if err != nil {
		return err
	}

	// Copy the contents of the 'regolith' folder to '.regolith/tmp'
	Logger.Debug("Copying project files to .regolith/tmp")
	// Avoid repetetive code of preparing ResourceFolder, BehaviorFolder
	// and DataPath with a closure
	setup_tmp_directory := func(
		path, short_name, descriptive_name string,
	) error {
		if path != "" {
			stats, err := os.Stat(path)
			if err != nil {
				if os.IsNotExist(err) {
					Logger.Warnf(
						"%s %q does not exist", descriptive_name, path)
					err = os.MkdirAll(
						fmt.Sprintf(".regolith/tmp/%s", short_name), 0666)
					if err != nil {
						return err
					}
				}
			} else if stats.IsDir() {
				err = copy.Copy(
					path, fmt.Sprintf(".regolith/tmp/%s", short_name),
					copy.Options{PreserveTimes: false, Sync: false})
				if err != nil {
					return err
				}
			} else { // The folder paths leads to a file
				return fmt.Errorf(
					"%s path %q is not a directory",
					descriptive_name, path)
			}
		} else {
			err = os.MkdirAll(
				fmt.Sprintf(".regolith/tmp/%s", short_name), 0666)
			if err != nil {
				return err
			}
		}
		return nil
	}

	err = setup_tmp_directory(config.ResourceFolder, "RP", "resource folder")
	if err != nil {
		return err
	}
	err = setup_tmp_directory(config.BehaviorFolder, "BP", "behavior folder")
	if err != nil {
		return err
	}
	err = setup_tmp_directory(profile.DataPath, "data", "data folder")
	if err != nil {
		return err
	}

	Logger.Debug("Setup done in ", time.Since(start))
	return nil
}

// RunProfile loads the profile from config.json and runs it. The profileName
// is the name of the profile which should be loaded from the configuration.
func RunProfile(profileName string) error {
	Logger.Info("Running profile: ", profileName)
	config := LoadConfig()

	profile := config.Profiles[profileName]

	// Check whether every filter, uses a supported filter type
	for _, f := range profile.Filters {
		err := f.Check()
		if err != nil {
			return err
		}
	}

	// Prepare tmp files
	err := SetupTmpFiles(*config, profile)
	if err != nil {
		return wrapError("Unable to setup profile", err)
	}

	// Run the filters!
	for filter := range profile.Filters {
		filter := profile.Filters[filter]
		path, _ := filepath.Abs(".")
		err := filter.Run(path)
		if err != nil {
			return wrapError(fmt.Sprintf("%s failed", filter.GetFriendlyName()), err)
		}
	}

	// Export files
	Logger.Info("Moving files to target directory")
	start := time.Now()
	err = ExportProject(profile, config.Name)
	if err != nil {
		return wrapError("Exporting project failed", err)
	}
	Logger.Debug("Done in ", time.Since(start))
	// Clear the tmp/data path
	err = os.RemoveAll(".regolith/tmp/data")
	if err != nil {
		return wrapError("Unable to clean .regolith/tmp/data directory", err)
	}
	return nil
}

// FilterCollectionFromFilterJson returns a collection of filters from a
// "filter.json" file of a remote filter.
func FilterCollectionFromFilterJson(path string) (*FilterCollection, error) {
	result := &FilterCollection{Filters: []FilterRunner{}}
	file, err := ioutil.ReadFile(path)

	if err != nil {
		return nil, wrapError(
			fmt.Sprintf("Couldn't read %q", path),
			err,
		)
	}

	var filterCollection map[string]interface{}
	err = json.Unmarshal(file, &filterCollection)
	if err != nil {
		Logger.Fatal("Couldn't load %s! Does the file contain correct json?", path, err)
	}
	// Filters
	filters, ok := filterCollection["filters"].([]interface{})
	if !ok {
		Logger.Fatalf("Could not parse filters of %q", path)
	}
	for i, filter := range filters {
		filter, ok := filter.(map[string]interface{})
		if !ok {
			Logger.Fatalf(
				"Could not parse filter %s of %q", i, path,
			)
		}
		result.Filters = append(result.Filters, RunnableFilterFromObject(filter))
	}
	return result, nil
}

// FilterCollection is a list of filters
type FilterCollection struct {
	Filters []FilterRunner `json:"filters,omitempty"`
}

type Profile struct {
	FilterCollection
	ExportTarget ExportTarget `json:"export,omitempty"`
	DataPath     string       `json:"dataPath,omitempty"`
}

func ProfileFromObject(profileName string, obj map[string]interface{}) Profile {
	result := Profile{}
	// Filters
	filters, ok := obj["filters"].([]interface{})
	if !ok {
		Logger.Fatalf("Could not parse filters of profile %q", profileName)
	}
	for i, filter := range filters {
		filter, ok := filter.(map[string]interface{})
		if !ok {
			Logger.Fatalf(
				"Could not parse filter %s of profile %q", i, profileName,
			)
		}
		result.Filters = append(result.Filters, RunnableFilterFromObject(filter))
	}
	// ExportTarget
	export, ok := obj["export"].(map[string]interface{})
	if !ok {
		Logger.Fatalf("Could not parse export property of profile %q", profileName)
	}
	result.ExportTarget = ExportTargetFromObject(export)
	// DataPath
	dataPath, ok := obj["dataPath"].(string)
	if !ok {
		Logger.Fatalf("Could not parse dataPath property of profile %q", profileName)
	}
	result.DataPath = dataPath
	return result
}

// Install installs all of the filters in the profile including the nested ones
func (p *Profile) Install(isForced bool) error {
	return p.installFilters(isForced, p.Filters, nil)
}

// installFilters provides a recursive function to install all filters in the
// profile. This function is not exposed outside of the regolith package. Use
// Install() instead.
func (p *Profile) installFilters(
	isForced bool, filters []FilterRunner, parentFilter *RemoteFilter,
) error {
	for _, filter := range filters {
		err := p.installFilter(isForced, filter, parentFilter)
		if err != nil {
			return err
		}
	}
	return nil
}

// installFilter installs a single filter.
// - Downloads the filter if it is remote
// - Installs dependencies
// - Copies the filter's data to the data folder
// - Handles additional filters within the 'filters.json' file
func (p *Profile) installFilter(
	isForced bool, filter FilterRunner, parentFilter *RemoteFilter,
) error {
	var err error

	if rf, ok := filter.(*RemoteFilter); ok {
		filterDirectory, err := rf.Download(isForced)
		if err != nil {
			return wrapError("could not download filter: ", err)
		}
		filterCollection, err := FilterCollectionFromFilterJson(
			filepath.Join(filterDirectory, "filter.json"))
		if err != nil {
			return fmt.Errorf(
				"could not load \"filter.json\" from path %q, while checking"+
					" for recursive dependencies", filterDirectory,
			)
		}
		p.installFilters(isForced, filterCollection.Filters, rf)
		rf.CopyFilterData(p)
	}
	filter.InstallDependencies(parentFilter)
	if err != nil {
		return wrapError("Could not download dependencies: ", err)
	}

	return nil
}
