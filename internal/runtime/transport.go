// transport.go — PR-05a-h4-c (AS-963) tui→runtime transport alias.
//
// The TUI used to expose `WithClients(*awsclient.ServiceClients)` as its
// AWS bootstrap option, forcing internal/tui to import internal/aws. After
// h4-c the option signature reads `*runtime.ServiceClients` — a transparent
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
