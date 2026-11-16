package cli

import (
 "bytes"
 "strings"
 "testing"
)

func TestCompletionShells(t *testing.T) {
 for _, shell := range []string{"bash","zsh","fish","powershell"} {
  t.Run(shell,func(t *testing.T){
   var out, errs bytes.Buffer
   if code:=Run([]string{"completion",shell},strings.NewReader(""),&out,&errs,"test");code!=0 {t.Fatalf("exit %d: %s",code,errs.String())}
   if !strings.Contains(out.String(),"socklens") || !strings.Contains(out.String(),"inspect") {t.Fatalf("missing completion data: %s",out.String())}
  })
 }
}
func TestCompletionRejectsUnknownShell(t *testing.T) {
 var out,errs bytes.Buffer
 if code:=Run([]string{"completion","unknown"},strings.NewReader(""),&out,&errs,"test");code!=1 {t.Fatalf("exit %d",code)}
}
