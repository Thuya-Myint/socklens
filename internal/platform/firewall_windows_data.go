package platform

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/Thuya-Myint/socklens/internal/firewall"
)

// Fixed read-only script. User-supplied peers, ports, names, or paths are never
// interpolated into PowerShell. Filters are joined by provider InstanceID.
const windowsFirewallScript = `
$ErrorActionPreference='Stop'
$ProgressPreference='SilentlyContinue'
[Console]::OutputEncoding=New-Object System.Text.UTF8Encoding($false)
function Index-Filters($items) {
 $index=@{}
 foreach($item in $items) {
  $key=[string]$item.InstanceID
  if(!$key -or $index.ContainsKey($key)){throw 'Ambiguous firewall filter identity'}
  $index[$key]=$item
 }
 return ,$index
}
$ports=Index-Filters @(Get-NetFirewallPortFilter -PolicyStore ActiveStore)
$addresses=Index-Filters @(Get-NetFirewallAddressFilter -PolicyStore ActiveStore)
$apps=Index-Filters @(Get-NetFirewallApplicationFilter -PolicyStore ActiveStore)
$services=Index-Filters @(Get-NetFirewallServiceFilter -PolicyStore ActiveStore)
$interfaces=Index-Filters @(Get-NetFirewallInterfaceFilter -PolicyStore ActiveStore)
$types=Index-Filters @(Get-NetFirewallInterfaceTypeFilter -PolicyStore ActiveStore)
$security=Index-Filters @(Get-NetFirewallSecurityFilter -PolicyStore ActiveStore)
$profiles=@(Get-NetFirewallProfile -PolicyStore ActiveStore | ForEach-Object {
 [pscustomobject]@{name=[string]$_.Name;enabled=[string]$_.Enabled;default_inbound=[string]$_.DefaultInboundAction;default_outbound=[string]$_.DefaultOutboundAction;detail='ActiveStore configuration; NotConfigured is unresolved. Profile/interface and rule enforcement must also apply.'}
})
$connections=@();$notes=@()
try {$connections=@(Get-NetConnectionProfile | ForEach-Object {[pscustomobject]@{name=[string]$_.InterfaceAlias;profile=[string]$_.NetworkCategory}})} catch {$notes+=('Active interface profiles unavailable: '+$_.Exception.Message)}
$rules=@(Get-NetFirewallRule -PolicyStore ActiveStore -Enabled True | ForEach-Object {
 $id=[string]$_.InstanceID;$p=$ports[$id];$a=$addresses[$id];$app=$apps[$id];$svc=$services[$id];$iface=$interfaces[$id];$type=$types[$id];$sec=$security[$id]
 $conditions=@('Enabled rule; profile and runtime enforcement still apply.',('Service='+[string]$svc.Service),('Package='+[string]$app.Package),('InterfaceType='+[string]$type.InterfaceType),('EnforcementStatus='+[string]$_.EnforcementStatus),('PrimaryStatus='+[string]$_.PrimaryStatus),('EdgeTraversalPolicy='+[string]$_.EdgeTraversalPolicy),('Security='+($sec | Select-Object Authentication,Encryption,OverrideBlockRules,LocalUser,RemoteUser,RemoteMachine | ConvertTo-Json -Compress)))
 if(!$p -or !$a -or !$app -or !$svc -or !$iface -or !$type -or !$sec){$conditions+='Some associated filters are unavailable; restrictions unresolved.'}
 [pscustomobject]@{id=[string]$_.Name;name=[string]$_.DisplayName;source='windows-active-store';direction=([string]$_.Direction).ToLowerInvariant();action=([string]$_.Action).ToLowerInvariant();family='any';protocols=@([string]$p.Protocol);local_ports=@($p.LocalPort | ForEach-Object {[string]$_});remote_ports=@($p.RemotePort | ForEach-Object {[string]$_});local_addresses=@($a.LocalAddress | ForEach-Object {[string]$_});remote_addresses=@($a.RemoteAddress | ForEach-Object {[string]$_});program=[Environment]::ExpandEnvironmentVariables([string]$app.Program);profiles=[string]$_.Profile;interfaces=(@($iface.InterfaceAlias) -join ',');conditions=$conditions}
})
[pscustomobject]@{profiles=$profiles;connections=$connections;rules=$rules;notes=$notes} | ConvertTo-Json -Depth 10 -Compress
`

func decodeWindowsFirewall(data []byte, s *firewall.Snapshot) error {
	var document struct {
		Profiles    []firewall.Profile `json:"profiles"`
		Connections []struct {
			Name    string `json:"name"`
			Profile string `json:"profile"`
		} `json:"connections"`
		Rules []firewall.Rule `json:"rules"`
		Notes []string        `json:"notes"`
	}
	if err := json.Unmarshal(data, &document); err != nil {
		return err
	}
	if document.Profiles == nil || document.Rules == nil {
		return fmt.Errorf("missing profiles or rules")
	}
	for i := range document.Rules {
		r := &document.Rules[i]
		if r.ID == "" || (r.Source != "windows-active-store" && r.Source != "windows-policy-com") {
			return fmt.Errorf("missing rule identity")
		}
		if r.Direction != "inbound" && r.Direction != "outbound" {
			r.Direction = "unknown"
		}
		if r.Action != "allow" && r.Action != "block" {
			r.Action = "unknown"
		}
		for _, field := range []*[]string{&r.Protocols, &r.LocalPorts, &r.RemotePorts, &r.LocalAddresses, &r.RemoteAddresses} {
			values := []string{}
			for _, value := range *field {
				for _, part := range strings.Split(value, ",") {
					if strings.TrimSpace(part) != "" {
						values = append(values, strings.TrimSpace(part))
					}
				}
			}
			*field = values
		}
	}
	s.Profiles = document.Profiles
	s.Rules = document.Rules
	s.Notes = append(s.Notes, document.Notes...)
	for i := range s.Interfaces {
		for _, connection := range document.Connections {
			if connection.Name == s.Interfaces[i].Name {
				s.Interfaces[i].Profile = connection.Profile
			}
		}
	}
	s.Notes = append(s.Notes, "Windows firewall policy collected with the indicated source. AppContainer, Hyper-V, third-party WFP filters, IPsec, active-profile enforcement and stateful response behavior are not fully evaluated.")
	return nil
}

// HNetCfg.FwPolicy2 exposes read-only policy/rule properties to ordinary users
// when bulk NetSecurity filter enumeration is denied. It is still partial:
// restricted service/AppContainer/WFP/Hyper-V filters are not enumerated here.
const windowsCOMFirewallScript = `
$ErrorActionPreference='Stop'
$ProgressPreference='SilentlyContinue'
[Console]::OutputEncoding=New-Object System.Text.UTF8Encoding($false)
$fw=New-Object -ComObject HNetCfg.FwPolicy2
function Action-Name($value){if([int]$value -eq 0){return 'block'};if([int]$value -eq 1){return 'allow'};return 'unknown'}
$active=[int]$fw.CurrentProfileTypes
$profiles=@(foreach($bit in @(1,2,4)) {
 $name=switch($bit){1{'Domain'};2{'Private'};4{'Public'}}
 [pscustomobject]@{name=$name;enabled=[string]$fw.FirewallEnabled($bit);default_inbound=(Action-Name $fw.DefaultInboundAction($bit));default_outbound=(Action-Name $fw.DefaultOutboundAction($bit));detail=('CurrentProfileTypes='+$active+'; profile active='+[bool]($active -band $bit)+'; BlockAllInboundTraffic='+[string]$fw.BlockAllInboundTraffic($bit)+'; observed COM properties, group policy and other layers may differ')}
})
$connections=@();$notes=@('COM fallback used; bulk ActiveStore filters were unavailable. Service hardening, app-container, IPsec and other WFP providers are not fully covered.')
try {$connections=@(Get-NetConnectionProfile | ForEach-Object {[pscustomobject]@{name=[string]$_.InterfaceAlias;profile=[string]$_.NetworkCategory}})} catch {$notes+=('Active interface profiles unavailable: '+$_.Exception.Message)}
$rules=@(foreach($r in $fw.Rules){
 if(!$r.Enabled){continue}
 $direction='unknown';if([int]$r.Direction -eq 1){$direction='inbound'};if([int]$r.Direction -eq 2){$direction='outbound'}
 $protocol='Any';if([int]$r.Protocol -ne 256){$protocol=[string]$r.Protocol}
 $localPorts=@();$remotePorts=@();if($protocol -eq '6' -or $protocol -eq '17'){$localPorts=@(([string]$r.LocalPorts) -split ',');$remotePorts=@(([string]$r.RemotePorts) -split ',')}
 $conditions=@(('Service='+[string]$r.ServiceName),('InterfaceTypes='+[string]$r.InterfaceTypes),('EdgeTraversal='+[string]$r.EdgeTraversal),'Enabled rule; active profile, service, identity, state and other restrictions are not resolved.')
 foreach($property in @('LocalAppPackageId','LocalUserAuthorizedList','RemoteUserAuthorizedList','RemoteMachineAuthorizedList','SecureFlags','EdgeTraversalOptions')){try{$conditions+=($property+'='+[string]$r.$property)}catch{$conditions+=($property+'=unavailable')}}
 [pscustomobject]@{id=[string]$r.Name;name=[string]$r.Name;source='windows-policy-com';direction=$direction;action=(Action-Name $r.Action);family='any';protocols=@($protocol);local_ports=$localPorts;remote_ports=$remotePorts;local_addresses=@(([string]$r.LocalAddresses) -split ',');remote_addresses=@(([string]$r.RemoteAddresses) -split ',');program=[Environment]::ExpandEnvironmentVariables([string]$r.ApplicationName);profiles=(@(foreach($profile in @(@{bit=1;name='Domain'},@{bit=2;name='Private'},@{bit=4;name='Public'})){if([int]$r.Profiles -band $profile.bit){$profile.name}}) -join ', ');interfaces=(@($r.Interfaces) -join ',');conditions=$conditions}
})
[pscustomobject]@{profiles=$profiles;connections=$connections;rules=$rules;notes=$notes} | ConvertTo-Json -Depth 10 -Compress
`
