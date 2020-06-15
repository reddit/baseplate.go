package filter

// Filter is a generic middleware type
type Filter interface {
	Do(request interface{}, service Service) (response interface{}, err error)
}

// Service is a generic client/server type
type Service interface {
	Do(request interface{}) (response interface{}, err error)
}

// ServiceWithFilters applies the filters to a service in a standard way.
func ServiceWithFilters(service Service, filters ...Filter) Service {
	for i := len(filters) - 1; i >= 0; i-- {
		service = &filteredService{filter: filters[i], service: service}
	}
	return service
}

type filteredService struct {
	filter  Filter
	service Service
}

func (fs *filteredService) Do(request interface{}) (response interface{}, err error) {
	return fs.filter.Do(request, fs.service)
}
