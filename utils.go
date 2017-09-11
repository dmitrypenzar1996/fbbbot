package main

func inGroup(userList []string, user string) (flag bool) {
	for _, value := range userList {
		if value == user {
			flag = true
			return
		}
	}
	flag = false
	return
}
