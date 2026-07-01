// transport.go — tui→runtime transport alias for the AWS ServiceClients.
//
// The TUI's WithClients bootstrap option takes `*runtime.ServiceClients` — a
// transparent
// alias for the AWS-typed ServiceClients struct. internal/runtime owns the
// alias because runtime already legitimately imports internal/aws; the TUI
// only sees the runtime-exported name so its production-side import set
// drops the awsclient package entirely.
package runtime

import awsclient "github.com/k2m30/a9s/v3/internal/aws"

// ServiceClients is the runtime-exported alias for *awsclient.ServiceClients.
// Renderer adapters reference this name in option signatures and accessor
// returns so they stay independent of internal/aws. The alias is intentional:
// any *awsclient.ServiceClients value flows through transparently with no
// conversion at the boundary.
type ServiceClients = awsclient.ServiceClients
