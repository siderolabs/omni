// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

package omni_test

import (
	"context"
	_ "embed"
	"fmt"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/jonboulle/clockwork"
	"github.com/siderolabs/talos/pkg/machinery/config/configloader"
	"github.com/siderolabs/talos/pkg/machinery/config/encoder"
	"github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/siderolabs/omni/client/pkg/omni/resources/omni"
	omnictrl "github.com/siderolabs/omni/internal/backend/runtime/omni/controllers/omni"
)

type RedactedClusterMachineConfigSuite struct {
	OmniSuite
}

//nolint:lll
func (suite *RedactedClusterMachineConfigSuite) TestReconcile() {
	ctx, cancel := context.WithTimeout(suite.ctx, 10*time.Second)
	defer cancel()

	suite.startRuntime()

	clock := clockwork.NewFakeClock()

	controller := omnictrl.NewRedactedClusterMachineConfigController(omnictrl.RedactedClusterMachineConfigControllerOptions{
		DiffCleanupInterval: 5 * time.Minute,
		DiffMaxAge:          15 * time.Minute,
		DiffMaxCount:        2,
		Clock:               clock,
	})

	suite.Require().NoError(suite.runtime.RegisterQController(controller))

	id := "test"

	cmc := omni.NewClusterMachineConfig(id)

	suite.Require().NoError(cmc.TypedSpec().Value.SetUncompressedData([]byte(`version: v1alpha1
machine:
  type: controlplane
  token: 02v8bh.y8uqauhyzpksn075
  ca:
    crt: LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSUJQakNCOGFBREFnRUNBaEEydVNNVDNETWhhc3VreUd1d3pZVXhNQVVHQXl0bGNEQVFNUTR3REFZRFZRUUsKRXdWMFlXeHZjekFlRncweU5UQTNNRGd4TWpFNE5EaGFGdzB6TlRBM01EWXhNakU0TkRoYU1CQXhEakFNQmdOVgpCQW9UQlhSaGJHOXpNQ293QlFZREsyVndBeUVBNU15S3FTY2RSUjJLRzBXS0dUTllrUjFmM0dBRkNtbVFvMTk5CmVsM0YwdUtqWVRCZk1BNEdBMVVkRHdFQi93UUVBd0lDaERBZEJnTlZIU1VFRmpBVUJnZ3JCZ0VGQlFjREFRWUkKS3dZQkJRVUhBd0l3RHdZRFZSMFRBUUgvQkFVd0F3RUIvekFkQmdOVkhRNEVGZ1FVRC9JQ0M4Mnl4QkFTOThRZQpaQzhneFVScUpVTXdCUVlESzJWd0EwRUFhSHM2S3Z1L0JDKzZzM2ZWQ1Y1NHRlQWpIZW5WTVdlcXFyb0V0bHBGCitDZXZQMlM3eHhXVU8zOTYzTjRxMFF1QzQvU2ZwVmFySzhmb1dKK0FBZ3pDQ3c9PQotLS0tLUVORCBDRVJUSUZJQ0FURS0tLS0tCg==
    key: LS0tLS1CRUdJTiBFRDI1NTE5IFBSSVZBVEUgS0VZLS0tLS0KTUM0Q0FRQXdCUVlESzJWd0JDSUVJRmN2TGVYT2pqaUJrQlZCQ1NwZkxxWVNGNVM5dGIraDVnd2Vjc3cyT3YrUAotLS0tLUVORCBFRDI1NTE5IFBSSVZBVEUgS0VZLS0tLS0K
cluster:
  id: 1vUXXJzS9ahM3TE70vm29k6weYtYgGDxxY-edDjvf_k=
  secret: n8eg5gx1fuumcig98VJqBE34CavL8XFGPGBw8M/p/MQ=
  controlPlane:
    endpoint: https://doesntmatter:6443
  token: l94i8b.8cl0f2k09h4tyga3
  secretboxEncryptionSecret: U3BEYHqEGj+SRiNRuWosaEjh4xHDrxhHT9TILBwFrVE=
  ca:
    crt: LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSUJpVENDQVMrZ0F3SUJBZ0lRSDV6TUR3SjhDdlpicEMwV2RZN2ZuakFLQmdncWhrak9QUVFEQWpBVk1STXcKRVFZRFZRUUtFd3ByZFdKbGNtNWxkR1Z6TUI0WERUSTFNRGN3T0RFeU1UZzBPRm9YRFRNMU1EY3dOakV5TVRnMApPRm93RlRFVE1CRUdBMVVFQ2hNS2EzVmlaWEp1WlhSbGN6QlpNQk1HQnlxR1NNNDlBZ0VHQ0NxR1NNNDlBd0VICkEwSUFCUFg2bE5CMXBNdFAzMzdRb3orZUVnaWgwMDIzTkEzRWczNVZmQldYdnJ6aG5SNkU0SXIyaHJkRDhzOFcKK1hMMWllUDdKUlFmWklORVBVVzZjeExNakR5allUQmZNQTRHQTFVZER3RUIvd1FFQXdJQ2hEQWRCZ05WSFNVRQpGakFVQmdnckJnRUZCUWNEQVFZSUt3WUJCUVVIQXdJd0R3WURWUjBUQVFIL0JBVXdBd0VCL3pBZEJnTlZIUTRFCkZnUVVrYmQvN1pFYWVrb0tIYVptdUVJMXVnN3d6QTR3Q2dZSUtvWkl6ajBFQXdJRFNBQXdSUUloQU56ZTFiNjEKdS9UV0tVU09mZ3JjVC9URTZYLytETGdDbXNDQU01OEg5Q3JtQWlCZlJXYktjVVpzWm9hOEZ6R1liNkNDL1V6bwozb3YwVDlSb2c3ZlJwM2tnaFE9PQotLS0tLUVORCBDRVJUSUZJQ0FURS0tLS0tCg==
    key: LS0tLS1CRUdJTiBFQyBQUklWQVRFIEtFWS0tLS0tCk1IY0NBUUVFSUJhY1NhdDdGeFNLb0lQb3ZnTmVkREx6MS9qQlpTTnB0ZUR4dWNuNjdoajNvQW9HQ0NxR1NNNDkKQXdFSG9VUURRZ0FFOWZxVTBIV2t5MC9mZnRDalA1NFNDS0hUVGJjMERjU0RmbFY4RlplK3ZPR2RIb1RnaXZhRwp0MFB5enhiNWN2V0o0L3NsRkI5a2cwUTlSYnB6RXN5TVBBPT0KLS0tLS1FTkQgRUMgUFJJVkFURSBLRVktLS0tLQo=
  aggregatorCA:
    crt: LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSUJZRENDQVFXZ0F3SUJBZ0lRTGRBcWdlUHQyUjg5dzZacTR5YUpmREFLQmdncWhrak9QUVFEQWpBQU1CNFgKRFRJMU1EY3dPREV5TVRnME9Gb1hEVE0xTURjd05qRXlNVGcwT0Zvd0FEQlpNQk1HQnlxR1NNNDlBZ0VHQ0NxRwpTTTQ5QXdFSEEwSUFCQktnK3BadnVGd1NBZUpEYXA0K29FeEdEY05tWFR6d3hPSmtjVXZLSHVXTnQxNnY3b3EvCmtSb2JXYTlnSVZHVTlVYTNXYXg5ekc1SnFKL1duZGpEblIrallUQmZNQTRHQTFVZER3RUIvd1FFQXdJQ2hEQWQKQmdOVkhTVUVGakFVQmdnckJnRUZCUWNEQVFZSUt3WUJCUVVIQXdJd0R3WURWUjBUQVFIL0JBVXdBd0VCL3pBZApCZ05WSFE0RUZnUVVxVy9kdW4wWGhETVgwaXUrTk5RbW50UHdvTEV3Q2dZSUtvWkl6ajBFQXdJRFNRQXdSZ0loCkFQWDNWM1R1TEdwZmc4Y21JWnFSMUZ2OFBWWE44cDgvaFR3Vk94clNMNlpkQWlFQTlCd0VzVGZDRWlUYm1vSFIKTmxPT3FmcndYQUtkZUxKeTJOZUdDdjZjV3JzPQotLS0tLUVORCBDRVJUSUZJQ0FURS0tLS0tCg==
    key: LS0tLS1CRUdJTiBFQyBQUklWQVRFIEtFWS0tLS0tCk1IY0NBUUVFSURBKzBtZEFmUUNQNEROa25pTXgvMGFzZUVOVFp2VjJJTGV6azVsaDR2QUtvQW9HQ0NxR1NNNDkKQXdFSG9VUURRZ0FFRXFENmxtKzRYQklCNGtOcW5qNmdURVlOdzJaZFBQREU0bVJ4UzhvZTVZMjNYcS91aXIrUgpHaHRacjJBaFVaVDFScmRackgzTWJrbW9uOWFkMk1PZEh3PT0KLS0tLS1FTkQgRUMgUFJJVkFURSBLRVktLS0tLQo=
  serviceAccount:
    key: LS0tLS1CRUdJTiBSU0EgUFJJVkFURSBLRVktLS0tLQpNSUlKS1FJQkFBS0NBZ0VBMmtYSThDVEUyekNDNXEwQXQ0cUFoSGdob05CeWJrZkpBUHpndE5GeHZ2ZzRTeDJXCmpBTGpHaS8rRlEyQmFzblM3V1NsVHNVdEJ1OGUrUkVPM3llQWpSMnZNdGVvRzh6dnJpQ3VKRjZHUktEaGNGMFgKZU1LRXp4VjR3WkdNQ1RhZnNSUXNoVXArSDlUUkw2QlkvTFpYbi9LUWtwUlNwMTBEbkVkaTlBbit4NjlJSWFqYQp0V1d5aU5Zckd2ZTJ1WVBBMWp2UWg0TUtiWmZBZFpkNzROYWw4K0JnaGRBUEVtNzlHOWcxYlg1RkNTampzVWxvCkFnNnB5VDJpVlEycDBKSjArU3VtcE04bWFSZ3NFejl2V1hVRC9GVjZmYzlXK1BnZUJvc0ZhM2NVcGQ0U29IT2gKcFBlcllLU0JVVzB3QzhoQ0xoT0ZRR2NmVWNYYXVzOFRWQk0wdEJIYzZ0V1dEK1AwTmt6Q3F5RUZMWVVzOUJxdwpsenB5YThVb2RObDFoSnZvS1B0VlBPRllOaU5jZGlXcXBrTXNJYjA5T2pZQjdOK2Y3cnNsbkJiM1VuOGJQb1J5CnN1cnY4eUN3L3hKTnpLbzdnYlAyT0VMY3FCZ0lXSVB1cS9sa2ZSK2VNd2I5eDJDeXdGOG4rYm1PMXgxNWtWNWcKV2tTZ0s5Unkva3lFdGxFZEx2NVVmc1FPWiszdjFLZnJGVmRPdGlYd1pDbndvcGRKTFVlWjl0ZENESFdlQjkwTwpheC9GR1BKT2QzTzZ5ZlVpOThHSHRTV0p1c3NwaXF6bkF0VkZqWlNtZEpQenVmb1Y2RU5QRTNoRHVrTXBnZkpFCnRRcjkyR1Jkc0thTDM2VUR5L0JYQ0ozRHoyN1RaZVJaYkdlQmpMRVpabG1BQUZndjRMOUQ2blRIYVFFQ0F3RUEKQVFLQ0FnQXdUeXUzQXR4VEN2eWQ0NEo2SFB4dTFVdlVGTzZPdS9LZjlsZ2hqUTJZejhWZDByR2tVV3RFTzRVSAowZEpuK1QxbTcxU3JCM2I4eHVYYkNFeDdWWG5kUWNtcC9oTWwvQWk4U0YxaWpVMDRXVWNzSUY1MmlzN3NLbnRzCmxETWpRdVM3UTVUSzkzN011c1NGdnY2VENDU0NzU1FRWFNXaUJ6TXFYcDVuRnVNOS9PeUJEcWRCYUwzSURXYkMKOURxTCtyNHViRlN0K1hIUWFicmVDK1lPRUZQd2t5T3AyaS9MeTZiWGg5WGpZd1FTaitzOUxOc0pRWWVRazhTZgpBSVFxTnBBUEtmc2JGUUlTVnBoQ3RsS0Z3U0Zkc3VtR2VPSnQvalJmREZ2cHVoUm0xYXpYdUYvNWJCdVJLemUzCjl1dWdYL1ZOejJJNXE5bEJ0d0cvUU4rdFJ3YnR2elVITnBBTUQ5eEZTR09jaGZFaXRyallKREVacTc0MmtqeGwKRkRXUGV5ZmtxVld6ZDRXUDl4MVNLWGlyZkpKOHJXWVNiN0N6c251WEJwTWp6TkswWEYwNkx6Y08vZDRlajExbwpuVXkrSGdUMC91SHpJY0ZaK2JvN2lRVkVVMEdPZFdheXJUVlVPYVZaY1ZnNUFseTdoYy90VXFHVnFrVERaWmFHCm9vUnpDVlg4R0ZISkdIdXRqU0Zrc1BnaDR1eUV6bGxsSGJYMUNXeld3cVNnVTJDSVVmYkExVzV5L08xSTRKMDAKZERYMDl5Lzc0ZTZrMTYvZkJGaXZ0Ulc2bFNYeUowNWwwUnVTNDMrWWpsU2hVSXI4RFBmRGRZTit0cW5KSVpGWQoxVjUrZGdkdU9NSXQ2dlQyTWVzeGhINWZnSUx6bHFXUDEyY2l5ajNSMXJIVHZYSXJNUUtDQVFFQTZXcEQxeEVtCjhQUHZWVXdXQm9XUno4M3NPQjhscWR1YWFCVGtFUWN0MU91U3M2U0U4c2hMNHpUWGRlSmk4b1BkL3ExRWQ1ZWgKUVFVbWxXbCtET0tNWnJrUHcwV0EwQVhFNGhFQkZQNENPWW1vMHByRkttVDJ6STVzRUEyT0RoVCthMzJYa2JKeApENHFhYlVnZGkvbldzTVNOWnJRcHVRc1dXVFZodUdCOVJDOUgzQUN2eVZvaEpmRDV5UWhWYlRRWDl2U0s4Mi9OClJtdHNyYVBaeW1Qa28vSjBUeDFEUEE4UUVkUkFycEhVR29CbXd0WnlVZDMxMjd4YmtUMzM2bmZMcjhQVEpNN2QKeXFrZXhBb2FCUm5SVmNZUWVRakVZUHRCNndWSUVubjdWTjlXalptRU9UY1E0MDhlYWg4cHpiUmFuS2NFaDg3SwpjSEhwSU1RR3lIWng5UUtDQVFFQTcyUnYrTG5HallOUnRBdXFjdnEzZVJiNDFHaklWSm9ZVXBmdG0wRjJldVVVCjZJQ2NUcUFwUi94a2NzVzQ0NVlkOFNxc3U5WUV3VlU4Um9ZZlJvVTJnRVg0UHhTeDlOZittZWdXeXFjS1hwRmEKM21BZ1dCNkl2Rit4UlZNV3JIL2dTVUpuWlNDcjgybVNZdDZyM0xmdVpUTERTcGJ6MmtXbXVpazVGd3NVcmZXTwpFa2xWMU14SFZPeXBjbUhjWml0K3hUNmJWVlc5R0RjQUlTNk9TV2diTEd1bnE5RWlDbis3WFNZcTlLRDRsa3gwCmk4bWU2ZmVCdVVSRndZQkhkS3RJalZxTjBTMkNHbUR4ajlheENGaWQxTTU2T1A0RjZTclpWeFI4M0xtenpkdlkKaEd6aVdxYVkrT0daa2FIWHZOTzAxTlk3WmMzRGJUWWFhZi9hN3BjWFhRS0NBUUVBMDJQcTVxYmhCbzFWTG9IRwozTWN4QStyeHlPM2tkVTJ1TEI3blljaUhxSEprblE4ZFhLY3JteXlyQ1ZjcTE0bTNqa09yWTBmT3dZMEJvWVUxCnBFTzBkZithRi9ZbEw4Qlp5NGNzM0s4aW9xdGFXc25TVUkrNXVBNHdMZVdveG5ZYTZJeUlyV25XM1FWZzBDSGsKcUhWdkN3NG5KV0Y2Kzl2ZnRKRVUzQjkrc3piQ3RLdG1pRXQ1QTl5V3k0c2htdEgzOWk4SWZHbS9sY3dLVThPMQpwWWNNZGJKSnhiQ3h5SDIzeHYzY1NuMUZnMjdRSWhxRzFEL1p2dFI2ZFRLVENTVFBNbkorRWJMTHlST2JDbDQvCnJHanlYZVVQM0IybGhGTnBJb2pZK2VyQlJOOHppdkFDZ0xLdk43M2F4SzlPYzc2bjVZR1pKOG1QSzREdWFqOCsKQ1dURDFRS0NBUUVBdi9xdTVTdXV0RndFa0x2dVJHa1Y3QkRsR2dxeDVVN3loSUg4ZGM2b3dtT21RZEtxQjAvZgo2eS9ZS2thd1FDdHA5YmJBY1o1dmo4L1lGOEtGb0Z0Q1d0cEIrK3lQemdmTjBRVlVDYzZ0dlNzYVVVMkxncjl4CjdvZGJOWG90cThhZFNvTHJRaWxTWEZGa3FNOWp5Z3pqTE5ycHpJNkVIcDVPMStvcE0zYWFiZXVIdE5pRThiT2sKM05FeURsMjJqMlVBTkJSQ0k4d3ZhaFRwa0xLeVB1SXpNSXRoR3FRTGhabnIyd2E1MmhhaFpIOEowL1NyOFh1ZwoxNytOcFdGSGJLUFQrakFOblJ1K3c0TE5GZ29aVE5Vc05iWWtSRUpLNFRPUXVvbmVuSEI4Wm5HUkVKbjFhTGRECjVBdWZ5UytlUUhzVEFNQ1JQOUlra0JlY1ZUZHZEbm15clFLQ0FRQlYzWmtNaFB0NUJMZ0lSRit5K3F1M3VBZVUKUTBPN2hhanpTdGZVa1JqQ2xoOFM5aERnWjRyTlZDYWkyTitnYnV6TTRIVFZJSUlSK1grV2dEQWV4NG1CTXk1eApHcFBOaDZ0VVlIamRPcSt6YzJRcEpCejZPazlscHFTRUZRSEl0c2tGamV4SFA1QXdteFFsa0xzb3NXQlJud3RIClBCSi9GcWtvNVMvMG8xS24yaHB1bGdUcit0V3ZaSkZMOTVpdzJScWZ6ZldRWjFVRG90SFZWRE12Vzg1U0t5Z3QKV2E0QjFtUWdZaWQwUVZZVE1jcGNEaXZzZEFqYktuZnMvM21JcUhJbWJSd2JvZEsxV1FNZlVqUDE2Y1NmUVc5NgpNS282Z0FUUi8xL2pVdElvWEdSUjlvOVBuYjQ4ZWlzcE1HVGp6M3o2Q3IvZ3hXbDlNczNTM2xRRjlzUWUKLS0tLS1FTkQgUlNBIFBSSVZBVEUgS0VZLS0tLS0K
  etcd:
    ca:
      crt: LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSUJmVENDQVNPZ0F3SUJBZ0lRRVEyU3FYYkhUc1cyQWVLejlZMlA2akFLQmdncWhrak9QUVFEQWpBUE1RMHcKQ3dZRFZRUUtFd1JsZEdOa01CNFhEVEkxTURjd09ERXlNVGcwT0ZvWERUTTFNRGN3TmpFeU1UZzBPRm93RHpFTgpNQXNHQTFVRUNoTUVaWFJqWkRCWk1CTUdCeXFHU000OUFnRUdDQ3FHU000OUF3RUhBMElBQkV6YkwyWjI2QlFzCmU2MHB6c3l4Wm1kK01FeFRrOUFLSUtGdVRBbmN4TWI5RE9CUHFwOE02ZFVyUnB5UUw2TTdVR1RxWkJGSlZYeUcKRGkyTXBGRVNWR3FqWVRCZk1BNEdBMVVkRHdFQi93UUVBd0lDaERBZEJnTlZIU1VFRmpBVUJnZ3JCZ0VGQlFjRApBUVlJS3dZQkJRVUhBd0l3RHdZRFZSMFRBUUgvQkFVd0F3RUIvekFkQmdOVkhRNEVGZ1FVM01nQ3c4RWFjbGY4CnFjZ1dJTHR5VWxMTGVYY3dDZ1lJS29aSXpqMEVBd0lEU0FBd1JRSWdVN3llYU90enIrTUZrU0dHR2NlbWNNUCsKd1dUSVFOSzk5M3FnZWJlZHVlQUNJUURDODhnSlIwU1kxOWhDNkhmNlhZeHdQMlNiL2pMUTRpc3IrdGxFTG5odwpXQT09Ci0tLS0tRU5EIENFUlRJRklDQVRFLS0tLS0K
      key: LS0tLS1CRUdJTiBFQyBQUklWQVRFIEtFWS0tLS0tCk1IY0NBUUVFSUpkTERQQ2p0QWdTaWtZRU5GcHRIcjNDbllaSHFnenZkYlFucGFjb25qaHVvQW9HQ0NxR1NNNDkKQXdFSG9VUURRZ0FFVE5zdlpuYm9GQ3g3clNuT3pMRm1aMzR3VEZPVDBBb2dvVzVNQ2R6RXh2ME00RStxbnd6cAoxU3RHbkpBdm96dFFaT3BrRVVsVmZJWU9MWXlrVVJKVWFnPT0KLS0tLS1FTkQgRUMgUFJJVkFURSBLRVktLS0tLQo=
`)))

	// t = 0

	// create the config, assert that the redacted config is created
	suite.Require().NoError(suite.state.Create(ctx, cmc))

	rtestutils.AssertResource(ctx, suite.T(), suite.state, id,
		func(cmcr *omni.RedactedClusterMachineConfig, assert *assert.Assertions) {
			buffer, err := cmcr.TypedSpec().Value.GetUncompressedData()
			assert.NoError(err)

			defer buffer.Free()

			data := string(buffer.Data())

			assert.Equal(`version: v1alpha1
machine:
    type: controlplane
    token: '******'
    ca:
        crt: LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSUJQakNCOGFBREFnRUNBaEEydVNNVDNETWhhc3VreUd1d3pZVXhNQVVHQXl0bGNEQVFNUTR3REFZRFZRUUsKRXdWMFlXeHZjekFlRncweU5UQTNNRGd4TWpFNE5EaGFGdzB6TlRBM01EWXhNakU0TkRoYU1CQXhEakFNQmdOVgpCQW9UQlhSaGJHOXpNQ293QlFZREsyVndBeUVBNU15S3FTY2RSUjJLRzBXS0dUTllrUjFmM0dBRkNtbVFvMTk5CmVsM0YwdUtqWVRCZk1BNEdBMVVkRHdFQi93UUVBd0lDaERBZEJnTlZIU1VFRmpBVUJnZ3JCZ0VGQlFjREFRWUkKS3dZQkJRVUhBd0l3RHdZRFZSMFRBUUgvQkFVd0F3RUIvekFkQmdOVkhRNEVGZ1FVRC9JQ0M4Mnl4QkFTOThRZQpaQzhneFVScUpVTXdCUVlESzJWd0EwRUFhSHM2S3Z1L0JDKzZzM2ZWQ1Y1NHRlQWpIZW5WTVdlcXFyb0V0bHBGCitDZXZQMlM3eHhXVU8zOTYzTjRxMFF1QzQvU2ZwVmFySzhmb1dKK0FBZ3pDQ3c9PQotLS0tLUVORCBDRVJUSUZJQ0FURS0tLS0tCg==
        key: '******'
    certSANs: []
cluster:
    id: 1vUXXJzS9ahM3TE70vm29k6weYtYgGDxxY-edDjvf_k=
    secret: '******'
    controlPlane:
        endpoint: https://doesntmatter:6443
    token: '******'
    secretboxEncryptionSecret: '******'
    ca:
        crt: LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSUJpVENDQVMrZ0F3SUJBZ0lRSDV6TUR3SjhDdlpicEMwV2RZN2ZuakFLQmdncWhrak9QUVFEQWpBVk1STXcKRVFZRFZRUUtFd3ByZFdKbGNtNWxkR1Z6TUI0WERUSTFNRGN3T0RFeU1UZzBPRm9YRFRNMU1EY3dOakV5TVRnMApPRm93RlRFVE1CRUdBMVVFQ2hNS2EzVmlaWEp1WlhSbGN6QlpNQk1HQnlxR1NNNDlBZ0VHQ0NxR1NNNDlBd0VICkEwSUFCUFg2bE5CMXBNdFAzMzdRb3orZUVnaWgwMDIzTkEzRWczNVZmQldYdnJ6aG5SNkU0SXIyaHJkRDhzOFcKK1hMMWllUDdKUlFmWklORVBVVzZjeExNakR5allUQmZNQTRHQTFVZER3RUIvd1FFQXdJQ2hEQWRCZ05WSFNVRQpGakFVQmdnckJnRUZCUWNEQVFZSUt3WUJCUVVIQXdJd0R3WURWUjBUQVFIL0JBVXdBd0VCL3pBZEJnTlZIUTRFCkZnUVVrYmQvN1pFYWVrb0tIYVptdUVJMXVnN3d6QTR3Q2dZSUtvWkl6ajBFQXdJRFNBQXdSUUloQU56ZTFiNjEKdS9UV0tVU09mZ3JjVC9URTZYLytETGdDbXNDQU01OEg5Q3JtQWlCZlJXYktjVVpzWm9hOEZ6R1liNkNDL1V6bwozb3YwVDlSb2c3ZlJwM2tnaFE9PQotLS0tLUVORCBDRVJUSUZJQ0FURS0tLS0tCg==
        key: '******'
    aggregatorCA:
        crt: LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSUJZRENDQVFXZ0F3SUJBZ0lRTGRBcWdlUHQyUjg5dzZacTR5YUpmREFLQmdncWhrak9QUVFEQWpBQU1CNFgKRFRJMU1EY3dPREV5TVRnME9Gb1hEVE0xTURjd05qRXlNVGcwT0Zvd0FEQlpNQk1HQnlxR1NNNDlBZ0VHQ0NxRwpTTTQ5QXdFSEEwSUFCQktnK3BadnVGd1NBZUpEYXA0K29FeEdEY05tWFR6d3hPSmtjVXZLSHVXTnQxNnY3b3EvCmtSb2JXYTlnSVZHVTlVYTNXYXg5ekc1SnFKL1duZGpEblIrallUQmZNQTRHQTFVZER3RUIvd1FFQXdJQ2hEQWQKQmdOVkhTVUVGakFVQmdnckJnRUZCUWNEQVFZSUt3WUJCUVVIQXdJd0R3WURWUjBUQVFIL0JBVXdBd0VCL3pBZApCZ05WSFE0RUZnUVVxVy9kdW4wWGhETVgwaXUrTk5RbW50UHdvTEV3Q2dZSUtvWkl6ajBFQXdJRFNRQXdSZ0loCkFQWDNWM1R1TEdwZmc4Y21JWnFSMUZ2OFBWWE44cDgvaFR3Vk94clNMNlpkQWlFQTlCd0VzVGZDRWlUYm1vSFIKTmxPT3FmcndYQUtkZUxKeTJOZUdDdjZjV3JzPQotLS0tLUVORCBDRVJUSUZJQ0FURS0tLS0tCg==
        key: '******'
    serviceAccount:
        key: '******'
    etcd:
        ca:
            crt: LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSUJmVENDQVNPZ0F3SUJBZ0lRRVEyU3FYYkhUc1cyQWVLejlZMlA2akFLQmdncWhrak9QUVFEQWpBUE1RMHcKQ3dZRFZRUUtFd1JsZEdOa01CNFhEVEkxTURjd09ERXlNVGcwT0ZvWERUTTFNRGN3TmpFeU1UZzBPRm93RHpFTgpNQXNHQTFVRUNoTUVaWFJqWkRCWk1CTUdCeXFHU000OUFnRUdDQ3FHU000OUF3RUhBMElBQkV6YkwyWjI2QlFzCmU2MHB6c3l4Wm1kK01FeFRrOUFLSUtGdVRBbmN4TWI5RE9CUHFwOE02ZFVyUnB5UUw2TTdVR1RxWkJGSlZYeUcKRGkyTXBGRVNWR3FqWVRCZk1BNEdBMVVkRHdFQi93UUVBd0lDaERBZEJnTlZIU1VFRmpBVUJnZ3JCZ0VGQlFjRApBUVlJS3dZQkJRVUhBd0l3RHdZRFZSMFRBUUgvQkFVd0F3RUIvekFkQmdOVkhRNEVGZ1FVM01nQ3c4RWFjbGY4CnFjZ1dJTHR5VWxMTGVYY3dDZ1lJS29aSXpqMEVBd0lEU0FBd1JRSWdVN3llYU90enIrTUZrU0dHR2NlbWNNUCsKd1dUSVFOSzk5M3FnZWJlZHVlQUNJUURDODhnSlIwU1kxOWhDNkhmNlhZeHdQMlNiL2pMUTRpc3IrdGxFTG5odwpXQT09Ci0tLS0tRU5EIENFUlRJRklDQVRFLS0tLS0K
            key: '******'
`, data)
		},
	)

	rtestutils.AssertResource(ctx, suite.T(), suite.state, id, func(res *omni.ClusterMachineConfig, assert *assert.Assertions) {
		assert.True(res.Metadata().Finalizers().Has(controller.Name()), "expected controller name finalizer to be set")
	})

	// no diffs should be saved on resource creation
	suite.assertDiffCount(ctx, 0)

	// update the config, it should generate a diff
	diffID1 := suite.updateConfigAssertDiff(ctx, clock, cmc, "aaa", "bbb")

	suite.assertDiffCount(ctx, 1)

	clock.Advance(1 * time.Minute)

	// t = 1m

	// update the config again, it should generate another diff
	diffID2 := suite.updateConfigAssertDiff(ctx, clock, cmc, "ccc", "ddd")
	suite.assertDiffCount(ctx, 2)

	clock.Advance(2 * time.Minute)

	// t = 3m

	// update the config again, it should generate a third diff
	diffID3 := suite.updateConfigAssertDiff(ctx, clock, cmc, "eee", "fff")
	suite.assertDiffCount(ctx, 3)

	// move the clock forward so that the periodic cleanup task gets triggered (at t = 5m)
	clock.Advance(3 * time.Minute)

	// t = 6m

	// assert that the oldest diff is removed, as we allow the max count of 2 diffs to be kept
	rtestutils.AssertNoResource[*omni.MachineConfigDiff](ctx, suite.T(), suite.state, diffID1)

	// assert that the other two diffs are still present
	rtestutils.AssertResource[*omni.MachineConfigDiff](ctx, suite.T(), suite.state, diffID2, func(*omni.MachineConfigDiff, *assert.Assertions) {})
	rtestutils.AssertResource[*omni.MachineConfigDiff](ctx, suite.T(), suite.state, diffID3, func(*omni.MachineConfigDiff, *assert.Assertions) {})

	// move the clock forward so that diff2 will be old enough to be removed (at t = 16m, it'll be 15m old)
	clock.Advance(11 * time.Minute)

	// t = 17m

	// assert that the second diff is removed, as it is older than 15 minutes
	rtestutils.AssertNoResource[*omni.MachineConfigDiff](ctx, suite.T(), suite.state, diffID2)

	// assert that the third diff is still present, as it is not old enough yet
	rtestutils.AssertResource[*omni.MachineConfigDiff](ctx, suite.T(), suite.state, diffID3, func(*omni.MachineConfigDiff, *assert.Assertions) {})

	// destroy the cluster machine config
	rtestutils.Destroy[*omni.ClusterMachineConfig](ctx, suite.T(), suite.state, []resource.ID{id})

	// assert that the redacted config is removed
	rtestutils.AssertNoResource[*omni.RedactedClusterMachineConfig](ctx, suite.T(), suite.state, id)

	// assert that the remaining diff is also removed
	rtestutils.AssertNoResource[*omni.MachineConfigDiff](ctx, suite.T(), suite.state, diffID3)

	// assert that there are no diffs left in the state
	suite.assertDiffCount(ctx, 0)
}

func (suite *RedactedClusterMachineConfigSuite) updateConfigAssertDiff(ctx context.Context, clock clockwork.Clock, cmc *omni.ClusterMachineConfig, testKey, testVal string) resource.ID {
	_, err := safe.StateUpdateWithConflicts(ctx, suite.state, cmc.Metadata(), func(res *omni.ClusterMachineConfig) error {
		data, err := res.TypedSpec().Value.GetUncompressedData()
		if err != nil {
			return err
		}

		defer data.Free()

		updatedConfig := suite.updateConfig(data.Data(), map[string]string{testKey: testVal})

		return res.TypedSpec().Value.SetUncompressedData(updatedConfig)
	})
	suite.Require().NoError(err)

	const rfc3339Millis = "2006-01-02T15:04:05.000Z07:00"

	expectedDiffID := cmc.Metadata().ID() + "-" + clock.Now().UTC().Format(rfc3339Millis)

	rtestutils.AssertResource[*omni.MachineConfigDiff](ctx, suite.T(), suite.state, expectedDiffID, func(res *omni.MachineConfigDiff, assertion *assert.Assertions) {
		diff := res.TypedSpec().Value.Diff

		assertion.Contains(diff, fmt.Sprintf("+        %s: %s", testKey, testVal))
	})

	return expectedDiffID
}

func (suite *RedactedClusterMachineConfigSuite) assertDiffCount(ctx context.Context, count int) {
	diffList, err := safe.StateListAll[*omni.MachineConfigDiff](ctx, suite.state)
	suite.Require().NoError(err)

	suite.Require().Equalf(count, diffList.Len(), "expected %d diffs, got %d", count, diffList.Len())
}

func (suite *RedactedClusterMachineConfigSuite) updateConfig(existingConfig []byte, nodeLabelsToAdd map[string]string) []byte {
	config, err := configloader.NewFromBytes(existingConfig)
	suite.Require().NoError(err)

	config, err = config.PatchV1Alpha1(func(config *v1alpha1.Config) error {
		config.MachineConfig.MachineNodeLabels = nodeLabelsToAdd

		return nil
	})
	suite.Require().NoError(err)

	encoded, err := config.EncodeBytes(encoder.WithComments(encoder.CommentsDisabled))
	suite.Require().NoError(err)

	return encoded
}

func TestRedactedClusterMachineConfigSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(RedactedClusterMachineConfigSuite))
}
