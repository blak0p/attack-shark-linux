package desktop

import (
	"context"
	"errors"

	"github.com/blak0p/attack-shark-linux/internal/mouse"
	"github.com/blak0p/attack-shark-linux/internal/x6"
)

func newSettingsState() *settingsState { return &settingsState{normalSleep: x6.NormalSleepMinimumMinutes, responseTimeMs: x6.ResponseTimeFactoryMs} }
func newSettingsStateFromConfig(c x6.DeviceConfig) *settingsState {
	s := newSettingsState()
	if x6.ValidateNormalSleepMinutes(c.NormalSleepMinutes) == nil { s.normalSleep = c.NormalSleepMinutes; v := c.NormalSleepMinutes; s.persistedNormal = &v; s.normalPersistence = "success" }
	if x6.ValidateResponseTime(c.ResponseTimeMs) == nil { s.responseTimeMs = c.ResponseTimeMs; v := c.ResponseTimeMs; s.persistedResponse = &v; s.responsePersistence = "success" }
	return s
}
func (s *settingsState) replace(c x6.DeviceConfig) {
	next := newSettingsStateFromConfig(c)
	s.mu.Lock()
	s.normalSleep, s.responseTimeMs = next.normalSleep, next.responseTimeMs
	s.persistedNormal, s.persistedResponse = next.persistedNormal, next.persistedResponse
	s.retryNormal, s.retryResponse = nil, nil
	s.normalRevision, s.responseRevision = next.normalRevision, next.responseRevision
	s.normalFirmware, s.responseFirmware = next.normalFirmware, next.responseFirmware
	s.normalPersistence, s.responsePersistence = next.normalPersistence, next.responsePersistence
	s.normalError, s.responseError = Error{}, Error{}
	s.mu.Unlock()
}
func (s *Service) currentSettingsState() *settingsState { b, ok := s.selectedBinding(); if !ok { return newSettingsState() }; s.mu.Lock(); defer s.mu.Unlock(); if s.settingsStates[b.ID] == nil { s.settingsStates[b.ID] = newSettingsState() }; return s.settingsStates[b.ID] }

func (s *Service) GetNormalSleepSnapshot() NormalSleepSnapshot { st:=s.currentSettingsState(); st.mu.Lock(); defer st.mu.Unlock(); return normalSleepSnapshot(st) }
func normalSleepSnapshot(st *settingsState) NormalSleepSnapshot { var p *float64; if st.persistedNormal != nil { v:=*st.persistedNormal; p=&v }; return NormalSleepSnapshot{Pending:st.normalSleep, Applied:st.normalSleep, Persisted:p, Revision:st.normalRevision, Firmware:st.normalFirmware, Persistence:st.normalPersistence, RetryAvailable:st.retryNormal!=nil, Error:st.normalError} }
func (s *Service) StageNormalSleep(minutes float64) NormalSleepSnapshot { st:=s.currentSettingsState(); st.mu.Lock(); defer st.mu.Unlock(); if x6.ValidateNormalSleepMinutes(minutes)!=nil { st.normalError=Error{Code:InvalidConfiguration}; return normalSleepSnapshot(st) }; st.normalSleep=minutes; st.normalRevision++; st.normalFirmware="pending"; st.normalError=Error{}; return normalSleepSnapshot(st) }
func (s *Service) ApplyNormalSleep() NormalSleepSnapshot { return s.applySettings(true) }
func (s *Service) RetryNormalSleepPersistence() NormalSleepSnapshot { return s.persistSettings(true) }

func (s *Service) GetDebounceSnapshot() DebounceSnapshot { st:=s.currentSettingsState(); st.mu.Lock(); defer st.mu.Unlock(); return debounceSnapshot(st) }
func debounceSnapshot(st *settingsState) DebounceSnapshot { var p *int; if st.persistedResponse != nil { v:=*st.persistedResponse; p=&v }; return DebounceSnapshot{Desired:st.responseTimeMs, Applied:st.responseTimeMs, Persisted:p, Factory:x6.ResponseTimeFactoryMs, Revision:st.responseRevision, Firmware:st.responseFirmware, Persistence:st.responsePersistence, RetryAvailable:st.retryResponse!=nil, Error:st.responseError} }
func (s *Service) StageDebounce(ms int) DebounceSnapshot { st:=s.currentSettingsState(); st.mu.Lock(); defer st.mu.Unlock(); if x6.ValidateResponseTime(ms)!=nil { st.responseError=Error{Code:InvalidConfiguration}; return debounceSnapshot(st) }; st.responseTimeMs=ms; st.responseRevision++; st.responseFirmware="pending"; st.responseError=Error{}; return debounceSnapshot(st) }
func (s *Service) ApplyDebounce() DebounceSnapshot { s.applySettings(false); return s.GetDebounceSnapshot() }
func (s *Service) RetryDebouncePersistence() DebounceSnapshot { s.persistSettings(false); return s.GetDebounceSnapshot() }

func (s *Service) applySettings(normal bool) NormalSleepSnapshot {
	b, ok:=s.selectedBinding(); st:=s.currentSettingsState(); if !ok { st.mu.Lock(); st.normalError=Error{Code:SelectionRequired}; defer st.mu.Unlock(); return normalSleepSnapshot(st) }
	st.applyMu.Lock(); defer st.applyMu.Unlock(); st.mu.Lock(); minutes, ms:=st.normalSleep,st.responseTimeMs; st.mu.Unlock()
	lighting:=s.currentLightingState(); lighting.mu.Lock(); selection:=lighting.pending; lighting.mu.Unlock()
	s.mu.Lock(); inventory:=s.inventory; s.mu.Unlock(); if inventory==nil || !s.bindingCurrent(b) { st.mu.Lock(); st.normalError=Error{Code:StaleBinding}; defer st.mu.Unlock(); return normalSleepSnapshot(st) }
	settings:=x6.LightingSettings{LightingSelection:selection, NormalSleepMinutes:minutes, ResponseTimeMs:ms}
	if err:=inventory.ApplyOperationBound(context.Background(),b,x6.NewLightingSettingsOperation(),settings); err!=nil { st.mu.Lock(); code:=ApplyFailed; if errors.Is(err,mouse.ErrStaleBinding){code=StaleBinding}; if normal {st.normalError=Error{Code:code};st.normalFirmware="failed"} else {st.responseError=Error{Code:code};st.responseFirmware="failed"}; defer st.mu.Unlock(); return normalSleepSnapshot(st) }
	st.mu.Lock(); if normal {st.normalFirmware="success";st.normalError=Error{}} else {st.responseFirmware="success";st.responseError=Error{}}; st.mu.Unlock(); return s.persistSettings(normal)
}
func (s *Service) persistSettings(normal bool) NormalSleepSnapshot { b,ok:=s.selectedBinding(); st:=s.currentSettingsState(); if !ok||b.SessionOnly { return s.GetNormalSleepSnapshot() }; s.mu.Lock(); p:=s.settingsPersistence; s.mu.Unlock(); if p==nil{return s.GetNormalSleepSnapshot()}; st.mu.Lock(); c:=x6.DeviceConfig{NormalSleepMinutes:st.normalSleep,ResponseTimeMs:st.responseTimeMs}; st.mu.Unlock(); if err:=p.Save(b,c);err!=nil {st.mu.Lock();if normal {v:=c.NormalSleepMinutes;st.retryNormal=&v;st.normalPersistence="failed";st.normalError=Error{Code:PersistenceFailed}} else {v:=c.ResponseTimeMs;st.retryResponse=&v;st.responsePersistence="failed";st.responseError=Error{Code:PersistenceFailed}};st.mu.Unlock();return s.GetNormalSleepSnapshot()}; st.mu.Lock();if normal {v:=c.NormalSleepMinutes;st.persistedNormal=&v;st.retryNormal=nil;st.normalPersistence="success"}else{v:=c.ResponseTimeMs;st.persistedResponse=&v;st.retryResponse=nil;st.responsePersistence="success"};st.mu.Unlock();return s.GetNormalSleepSnapshot() }
