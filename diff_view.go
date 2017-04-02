package main

import (
	"bufio"
	"bytes"
	log "github.com/Sirupsen/logrus"
	gc "github.com/rthornton128/goncurses"
	"sync"
)

type DiffViewHandler func(*DiffView) error

type DiffLine struct {
	line string
}

type Diff struct {
	lines   []*DiffLine
	viewPos *ViewPos
}

type DiffView struct {
	channels      *Channels
	repoData      RepoData
	activeCommit  *Commit
	commitDiffs   map[*Commit]*Diff
	viewPos       *ViewPos
	viewDimension ViewDimension
	handlers      map[gc.Key]DiffViewHandler
	active        bool
	lock          sync.Mutex
}

func NewDiffView(repoData RepoData, channels *Channels) *DiffView {
	return &DiffView{
		repoData:    repoData,
		channels:    channels,
		viewPos:     NewViewPos(),
		commitDiffs: make(map[*Commit]*Diff),
		handlers: map[gc.Key]DiffViewHandler{
			gc.KEY_UP:    MoveUpLine,
			gc.KEY_DOWN:  MoveDownLine,
			gc.KEY_RIGHT: ScrollDiffViewRight,
			gc.KEY_LEFT:  ScrollDiffViewLeft,
		},
	}
}

func (diffView *DiffView) Initialise() (err error) {
	return
}

func (diffView *DiffView) Render(win RenderWindow) (err error) {
	diffView.lock.Lock()
	defer diffView.lock.Unlock()

	diffView.viewDimension = win.ViewDimensions()

	if diffView.activeCommit == nil {
		return
	}

	rows := win.Rows() - 2
	viewPos := diffView.viewPos
	viewPos.DetermineViewStartRow(rows)

	diff := diffView.commitDiffs[diffView.activeCommit]
	lineNum := uint(len(diff.lines))
	lineIndex := viewPos.viewStartRowIndex
	startColumn := viewPos.viewStartColumn

	for rowIndex := uint(0); rowIndex < rows && lineIndex < lineNum; rowIndex++ {
		if err = win.SetRow(rowIndex+1, startColumn, " %v", diff.lines[lineIndex].line); err != nil {
			return
		}

		lineIndex++
	}

	if err = win.SetSelectedRow((viewPos.activeRowIndex-viewPos.viewStartRowIndex)+1, diffView.active); err != nil {
		return
	}

	win.DrawBorder()

	if err = win.SetTitle(CMP_COMMITVIEW_TITLE, "Diff for commit %v", diffView.activeCommit.commit.Id().String()); err != nil {
		return
	}

	if err = win.SetFooter(CMP_COMMITVIEW_FOOTER, "Line %v of %v", viewPos.activeRowIndex+1, lineNum); err != nil {
		return
	}

	return
}

func (diffView *DiffView) OnActiveChange(active bool) {
	log.Debugf("DiffView active: %v", active)
	diffView.lock.Lock()
	defer diffView.lock.Unlock()

	diffView.active = active
}

func (diffView *DiffView) OnCommitSelect(commit *Commit) (err error) {
	diffView.lock.Lock()
	defer diffView.lock.Unlock()

	if diff, ok := diffView.commitDiffs[diffView.activeCommit]; ok {
		diff.viewPos = diffView.viewPos
	}

	if diff, ok := diffView.commitDiffs[commit]; ok {
		diffView.activeCommit = commit
		diffView.viewPos = diff.viewPos
		diffView.channels.UpdateDisplay()
		return
	}

	buf, err := diffView.repoData.Diff(commit)
	if err != nil {
		return
	}

	scanner := bufio.NewScanner(bytes.NewReader(buf.Bytes()))
	var lines []*DiffLine

	for scanner.Scan() {
		lines = append(lines, &DiffLine{
			line: scanner.Text(),
		})
	}

	diffView.commitDiffs[commit] = &Diff{
		lines: lines,
	}

	diffView.activeCommit = commit
	diffView.viewPos = NewViewPos()
	diffView.channels.UpdateDisplay()

	return
}

func (diffView *DiffView) Handle(keyPressEvent KeyPressEvent) (err error) {
	log.Debugf("DiffView handling key %v", keyPressEvent)
	diffView.lock.Lock()
	defer diffView.lock.Unlock()

	if handler, ok := diffView.handlers[keyPressEvent.key]; ok {
		err = handler(diffView)
	}

	return
}

func MoveDownLine(diffView *DiffView) (err error) {
	diff := diffView.commitDiffs[diffView.activeCommit]
	lineNum := uint(len(diff.lines))
	viewPos := diffView.viewPos

	if viewPos.MoveLineDown(lineNum) {
		log.Debugf("Moving down one line in diff view")
		diffView.channels.UpdateDisplay()
	}

	return
}

func MoveUpLine(diffView *DiffView) (err error) {
	viewPos := diffView.viewPos

	if viewPos.MoveLineUp() {
		log.Debugf("Moving up one line in diff view")
		diffView.channels.UpdateDisplay()
	}

	return
}

func ScrollDiffViewRight(diffView *DiffView) (err error) {
	viewPos := diffView.viewPos
	viewPos.MovePageRight(diffView.viewDimension.cols)
	log.Debugf("Scrolling right. View starts at column %v", viewPos.viewStartColumn)
	diffView.channels.UpdateDisplay()

	return
}

func ScrollDiffViewLeft(diffView *DiffView) (err error) {
	viewPos := diffView.viewPos

	if viewPos.MovePageLeft(diffView.viewDimension.cols) {
		log.Debugf("Scrolling left. View starts at column %v", viewPos.viewStartColumn)
		diffView.channels.UpdateDisplay()
	}

	return
}