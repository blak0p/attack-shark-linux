package desktop

import (
	"context"
	"errors"
	"io"
	"reflect"
	"testing"
	"time"

	"github.com/blak0p/attack-shark-linux/internal/update"
)

func TestApplyVerifiedUpdateRetriesOnlyRelaunchAfterReplacement(t *testing.T) {
	service := &Service{}
	applies, launches := 0, 0
	ConfigureUpdater(service, nil, func() error {
		launches++
		if launches == 1 {
			return errors.New("launch failed")
		}
		return nil
	})
	service.update.apply = func(context.Context, *update.VerifiedUpdate, bool) error {
		applies++
		return nil
	}
	service.update.verified = &update.VerifiedUpdate{}

	if err := service.ApplyVerifiedUpdate(context.Background()); err == nil || err.Error() != "launch failed" {
		t.Fatalf("first ApplyVerifiedUpdate() error = %v, want launch failure", err)
	}
	if err := service.ApplyVerifiedUpdate(context.Background()); err != nil {
		t.Fatalf("retry ApplyVerifiedUpdate() error = %v", err)
	}
	if applies != 1 || launches != 2 {
		t.Fatalf("apply/relaunch calls = %d/%d, want 1/2", applies, launches)
	}
}

type cancellationTransport struct{ started chan struct{} }

func (transport cancellationTransport) Manifests(ctx context.Context) ([]update.Manifest, error) {
	close(transport.started)
	<-ctx.Done()
	return nil, ctx.Err()
}

func (cancellationTransport) Download(context.Context, string) (io.ReadCloser, error) {
	return nil, errors.New("unexpected download")
}

func TestDesktopUpdateOperationsPropagateCallerCancellation(t *testing.T) {
	checkStarted := make(chan struct{})
	service := &Service{}
	ConfigureUpdater(service, &update.Updater{
		CurrentVersion: "1.2.0",
		Transport:      cancellationTransport{started: checkStarted},
	}, nil)
	checkContext, cancelCheck := context.WithCancel(context.Background())
	checkResult := make(chan error, 1)
	go func() {
		_, err := service.CheckForUpdate(checkContext)
		checkResult <- err
	}()
	<-checkStarted
	cancelCheck()
	if err := <-checkResult; !errors.Is(err, context.Canceled) {
		t.Fatalf("CheckForUpdate() error = %v, want context.Canceled", err)
	}

	applyStarted := make(chan struct{})
	service.update.verified = &update.VerifiedUpdate{}
	service.update.apply = func(ctx context.Context, _ *update.VerifiedUpdate, _ bool) error {
		close(applyStarted)
		<-ctx.Done()
		return ctx.Err()
	}
	applyContext, cancelApply := context.WithCancel(context.Background())
	applyResult := make(chan error, 1)
	go func() { applyResult <- service.ApplyVerifiedUpdate(applyContext) }()
	<-applyStarted
	cancelApply()
	if err := <-applyResult; !errors.Is(err, context.Canceled) {
		t.Fatalf("ApplyVerifiedUpdate() error = %v, want context.Canceled", err)
	}
}

func TestApplyVerifiedUpdateHasLongerBoundThanCheck(t *testing.T) {
	service := &Service{}
	ConfigureUpdater(service, nil, nil)
	service.update.check = func(ctx context.Context) (*update.VerifiedUpdate, error) {
		deadline, ok := ctx.Deadline()
		if !ok || time.Until(deadline) > 15*time.Second || time.Until(deadline) < 14*time.Second {
			t.Errorf("check deadline = %v (present: %v), want about 15s", deadline, ok)
		}
		return nil, update.ErrNoUpdate
	}
	service.update.apply = func(ctx context.Context, _ *update.VerifiedUpdate, approved bool) error {
		deadline, ok := ctx.Deadline()
		remaining := time.Until(deadline)
		if !ok || remaining < 2*time.Minute || remaining > 3*time.Minute || !approved {
			t.Errorf("apply deadline remaining = %v (present: %v, approved: %v), want 2-3m", remaining, ok, approved)
		}
		return nil
	}
	if _, err := service.CheckForUpdate(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := service.ApplyVerifiedUpdate(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestDesktopUpdateMethodsAcceptWailsContext(t *testing.T) {
	serviceType := reflect.TypeOf(&Service{})
	contextType := reflect.TypeOf((*context.Context)(nil)).Elem()
	for _, methodName := range []string{"CheckForUpdate", "ApplyVerifiedUpdate"} {
		method, found := serviceType.MethodByName(methodName)
		if !found || method.Type.NumIn() != 2 || method.Type.In(1) != contextType {
			t.Errorf("%s must accept exactly one Wails-injected context.Context", methodName)
		}
	}
}

func TestServiceDoesNotExposeUpdateCompositionOrRecovery(t *testing.T) {
	serviceType := reflect.TypeOf(&Service{})
	for _, method := range []string{"AttachUpdater", "RecoverUpdates"} {
		if _, exported := serviceType.MethodByName(method); exported {
			t.Errorf("Service must not expose %s to Wails bindings", method)
		}
	}
}
