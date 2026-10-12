package main

type RouteGroup struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

type GlobalVariable struct {
	ID          string `json:"id"`
	KeyName     string `json:"key_name"`
	ValueData   string `json:"value_data"`
	IsSecret    bool   `json:"is_secret"`
	Description string `json:"description"`
}

type Endpoint struct {
	ID       string `json:"id"`
	RouteID  string `json:"route_id"`
	Role     string `json:"role"`
	Protocol string `json:"protocol"`
	Format   string `json:"format"`
	Config   string `json:"config"`
}

type Transformer struct {
	ID               string `json:"id"`
	RouteID          string `json:"route_id"`
	SourceScript     string `json:"source_script"`
	Script           string `json:"script"`
	InboundTemplate  string `json:"inbound_template"`
	OutboundTemplate string `json:"outbound_template"`
}

type Route struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	GroupName   string            `json:"group_name"`
	Active      bool              `json:"active"`
	Variables   map[string]string `json:"variables"`
	Endpoints   []Endpoint        `json:"endpoints"`
	Transformer Transformer       `json:"transformer"`
}

type MessageLog struct {
	ID             string `json:"id"`
	RouteID        string `json:"route_id"`
	RouteName      string `json:"route_name"`
	SourceProtocol string `json:"source_protocol"`
	TargetProtocol string `json:"target_protocol"`
	InboundData    string `json:"inbound_data"`
	OutboundData   string `json:"outbound_data"`
	ResponseData   string `json:"response_data"`
	Status         string `json:"status"`
	ErrorMessage   string `json:"error_message"`
	CreatedAt      string `json:"created_at"`
}

type User struct {
	ID           string `json:"id"`
	Username     string `json:"username"`
	PasswordHash string `json:"-"`
	Active       bool   `json:"active"`
}
