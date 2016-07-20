package Gpss

import (
	"time"
	"encoding/json"
	"github.com/garyburd/redigo/redis"
	"github.com/pmylund/sortutil"
	"strings"
	"errors"
)

type GpsDP struct {
	HubId     int64
	NodeId    int64
	TimeStamp time.Time
	Lat float64 `validate:"required,gte=-90,lte=90"`
	Lng float64 `validate:"required,gte=-180,lte=180"`
	Speed int
	Offset int
}

var rds redis.Conn

func Connect(addr string, pwd string) {
	c, err := redis.Dial("tcp", addr)
	if err != nil {
		panic(err)
	}
	_, err = c.Do("AUTH", pwd)
	if err != nil {
		panic(err)
	}
	rds = c
	rds.Do("SELECT", "6")
}

func GpsCreate(k string, dp *GpsDP) error {
	json, err := json.Marshal(dp)
	if err != nil {
		return err
	}
	_, err = rds.Do("SET", k, json)
	if err != nil {
		return err
	}
	return nil
}

func MaxGet(k string) (*GpsDP, error) {
	data, err := redis.Strings(rds.Do("KEYSSTART", k))
	if err != nil {
		return nil, err
	}
	var ndata []*GpsDP
	for _, v := range data {
		o, _ := redis.String(rds.Do("GET", v))
		h := &GpsDP{}
		json.Unmarshal([]byte(o), &h)
		ndata = append(ndata, h)
	}
	sortutil.DescByField(ndata, "TimeStamp")
	return ndata[0], nil
}

func GetOneByKey(k string) (*GpsDP, error) {
	o, err := redis.String(rds.Do("GET", k))
	if err != nil {
		return nil, err
	}
	h := &GpsDP{}
	json.Unmarshal([]byte(o), &h)
	return h, nil
}

func Replace(k string, h *GpsDP) error {
	json, err := json.Marshal(h)
	if err != nil {
		return err
	}
	_, err = rds.Do("SET", k, json)
	if err != nil {
		return err
	}
	return nil
}

func Delete(k string) error {
	if len(strings.TrimSpace(k)) == 0 {
		return errors.New("uid was nil")
	}
	_, err := redis.Int(rds.Do("DEL", k))
	if err != nil {
		return err
	}
	return nil
}

func GetRange(start string, end string, interval float64, page int) ([]*GpsDP, error) {
	data, err := redis.Strings(rds.Do("KEYSRANGE", start, end))
	if err != nil {
		return nil, err
	}
	sortutil.Desc(data)
	var IntervalData []string
	for _, v := range data {
		if len(IntervalData) == 0 {
			IntervalData = append(IntervalData, v)
		} else {
			otime := strings.Split(IntervalData[len(IntervalData) - 1], ",")
			otm, _ := time.Parse(time.RFC3339Nano, otime[3])
			vtime := strings.Split(v, ",")
			vtm, _ := time.Parse(time.RFC3339Nano, vtime[3])
			du := otm.Sub(vtm)
			if du.Seconds() >= interval{
				IntervalData = append(IntervalData, v)
			}
		}
	}
	pageSize := 50
	allcount := len(IntervalData)
	lastPageSize := allcount % pageSize
	totalPage := (allcount + pageSize - 1) / pageSize
	if page > totalPage {
		return nil, errors.New("pages out of range")
	}
	var pageData []string
	if page == 1 {
		if totalPage == page {
			//只有一页
			pageData = IntervalData[:allcount]
		} else {
			//不止一页的第一页
			pageData = IntervalData[:pageSize]
		}
	} else if page < totalPage {
		//中间页
		cursor := (pageSize * page) - pageSize //起启位计算
		pageData = IntervalData[cursor:cursor + pageSize]
	} else if page == totalPage {
		//尾页
		if lastPageSize == 0 {
			pageData = IntervalData[allcount - pageSize:]
		} else {
			pageData = IntervalData[allcount - lastPageSize:]
		}
	} else {
		return nil, errors.New("page not ext")
	}
	var ndata []*GpsDP
	for _, v := range pageData {
		o, _ := redis.String(rds.Do("GET", v))
		h := &GpsDP{}
		json.Unmarshal([]byte(o), &h)
		ndata = append(ndata, h)
	}
	sortutil.DescByField(ndata, "TimeStamp")
	return ndata, nil
}