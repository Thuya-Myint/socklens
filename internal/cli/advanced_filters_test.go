package cli

import (
 "bytes"
 "strings"
 "testing"
 "github.com/Thuya-Myint/socklens/internal/platform"
)

func TestAdvancedSocketFilters(t *testing.T) {
 rows:=[]platform.Socket{
  {Protocol:"tcp",Local:"127.0.0.1:8080",State:"LISTEN",Processes:[]platform.Process{{PID:1,Name:"Node.exe"}}},
  {Protocol:"tcp",Local:"[::1]:3000",State:"ESTABLISHED",Processes:[]platform.Process{{PID:2,Name:"python"}}},
  {Protocol:"udp",Local:"0.0.0.0:5353",State:"BOUND"},
 }
 cases:=[]struct{name string; filter socketFilter; want int}{
  {"process",socketFilter{process:"NODE"},1},
  {"process unknown",socketFilter{process:"ruby"},0},
  {"range",socketFilter{portRange:"3000-8080"},3},
  {"state",socketFilter{state:"established"},1},
  {"address",socketFilter{localAddress:"::1"},1},
  {"combined",socketFilter{portRange:"8000-9000",process:"node",state:"listen",localAddress:"127.0.0.1"},1},
 }
 for _,tc:=range cases { t.Run(tc.name,func(t *testing.T){ got,err:=filterSockets(rows,tc.filter);if err!=nil||len(got)!=tc.want {t.Fatalf("got %d, err %v, want %d",len(got),err,tc.want)} }) }
}
func TestAdvancedFilterValidation(t *testing.T) {
 for _,args:=range [][]string{
  {"list","--port-range","0-200"},
  {"list","--port-range","9000-8000"},
  {"list","--port-range","garbage"},
  {"list","--local-address","not-an-ip"},
  {"list","--port","80","--port-range","100-200"},
 } {
  var out,errs bytes.Buffer
  if code:=Run(args,strings.NewReader(""),&out,&errs,"test");code!=1 {t.Fatalf("%v accepted: %s",args,out.String())}
 }
}
