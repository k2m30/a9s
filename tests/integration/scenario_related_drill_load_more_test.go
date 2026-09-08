//go:build integration

package integration

import "testing"

// TestScenario_RelatedDrill_LoadMoreStaysOnTheDrill drives the one shape a
// continuation can get wrong, through the keys an operator presses: a related
// drill whose target type is truncated in demo, so the drill itself offers
// "m".
//
// The load-more must resolve the drill's own truncation and leave the operator
// on the drill: the title keeps naming the environment the log groups were
// built from, and its count loses the "+" without becoming the account's whole
// log-group population. A continuation that claims the canonical lane instead
// is refused by the drill screen and lands on the type's top-level list, where
// from the operator's seat the key either does nothing or silently rewrites
// the list underneath.
func TestScenario_RelatedDrill_LoadMoreStaysOnTheDrill(t *testing.T) {
	scenario := fullIntegrationNewDemoScenario(t)
	env := fullIntegrationMustFindResourceByNameContains(t, scenario.clients, "mwaa", "prod-airflow-etl")

	scenario.OpenDetailResource("mwaa", env)
	scenario.ExpectRelatedRow("Log Groups")

	scenario.FollowRelated("Log Groups")
	scenario.ExpectCurrentListType("logs")
	scenario.ExpectNoAPIError()
	// The drill opened on a truncated page: the "+" is what offers "m".
	scenario.ExpectFrameContains("logs(5+) -- " + env.Name)

	scenario.LoadMore()

	scenario.ExpectNoAPIError()
	scenario.ExpectCurrentListType("logs")
	scenario.ExpectFrameContains("logs(5) -- " + env.Name)
}
