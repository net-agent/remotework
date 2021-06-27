package notify

import "fmt"

type EmitArgs struct {
	TargetHostName string
	Message        string
}

type EmitReply struct {
	Desc string
}

func (n *Notify) Emit(args *EmitArgs, reply *EmitReply) error {
	reply.Desc = fmt.Sprintf("emit [%v][%v]", args.TargetHostName, args.Message)
	return nil
}
