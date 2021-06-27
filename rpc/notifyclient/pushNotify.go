package notifyclient

import "log"

type PushNotifyArgs struct {
	Sender  string
	Message string
}

type PushNotifyReply struct {
}

func (client *NotifyClient) PushNotify(args *PushNotifyArgs, replay *PushNotifyReply) error {
	log.Printf("[notify][%v]%v\n", args.Sender, args.Message)
	return nil
}
